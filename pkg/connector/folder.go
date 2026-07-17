package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"

	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-workato/pkg/connector/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

const (
	collaboratorAccessEntitlement = "collaborator-access"
)

var _ connectorbuilder.ResourceSyncerV2 = (*folderBuilder)(nil)

type folderBuilder struct {
	client                 *client.WorkatoClient
	cache                  *collaboratorCache
	disableCustomRolesSync bool
}

func (o *folderBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return folderResourceType
}

// List returns all the folders and project folders.
func (o *folderBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, attr rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	rv := make([]*v2.Resource, 0)

	// At the top level (nil parent) the SDK asks for every folder in the
	// workspace. Each Workato project owns one root folder, so we paginate
	// projects once and emit a root folder for each, parented to its own
	// project. Nested folders are discovered afterward because every folder
	// carries a ChildResourceType: folder annotation, which drives the
	// folderResourceType.Id branch below.
	if parentResourceID == nil {
		projects, nextToken, annos, err := o.client.GetProjects(ctx, attr.PageToken.Token)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: annos}, err
		}

		for _, project := range projects {
			projectParentID := &v2.ResourceId{
				ResourceType: projectResourceType.Id,
				Resource:     strconv.Itoa(project.Id),
			}
			rootFolder, err := projectFolderResource(project, projectParentID)
			if err != nil {
				return nil, &rs.SyncOpResults{Annotations: annos}, err
			}
			rv = append(rv, rootFolder)
		}

		return rv, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil
	}

	switch parentResourceID.ResourceType {
	case folderResourceType.Id:
		parentId, err := strconv.Atoi(parentResourceID.Resource)
		if err != nil {
			return nil, nil, err
		}

		folders, nextToken, annos, err := o.client.GetFolders(ctx, &parentId, attr.PageToken.Token)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: annos}, err
		}

		for _, folder := range folders {
			us, err := folderResource(folder, parentResourceID)
			if err != nil {
				return nil, &rs.SyncOpResults{Annotations: annos}, err
			}
			rv = append(rv, us)
		}

		return rv, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil

	default:
		l.Debug("baton-workato: unexpected parent resource type in folder List", zap.String("parent_resource_type", parentResourceID.ResourceType))
		return nil, nil, nil
	}
}

// Entitlements returns an entitlement for the folder to be assigned to a collaborator.
func (o *folderBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	var rv []*v2.Entitlement

	assigmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("%s can access %s", collaboratorResourceType.DisplayName, resource.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s access %s", collaboratorResourceType.DisplayName, resource.DisplayName)),
	}
	rv = append(rv, entitlement.NewPermissionEntitlement(resource, collaboratorAccessEntitlement, assigmentOptions...))

	return rv, nil, nil
}

// Grants returns the roles granted to a folder.
func (o *folderBuilder) Grants(ctx context.Context, resource *v2.Resource, attr rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	if o.disableCustomRolesSync {
		return nil, nil, nil
	}

	var rv []*v2.Grant

	folderId := resource.Id.Resource
	folderRoles, err := getRoleByFolder(ctx, attr.Session, folderId)
	if err != nil {
		return nil, nil, err
	}
	var healResults *rs.SyncOpResults
	if len(folderRoles) == 0 {
		// No mapping found. This is either a folder with no roles or an empty
		// roles cache for this sync session (e.g. a resumed sync that skipped
		// re-listing roles). ensureRolesCache is a no-op once the cache is
		// populated, so this only fetches when the cache is genuinely missing.
		annos, err := ensureRolesCache(ctx, attr.Session, o.client)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: annos}, err
		}
		if len(annos) > 0 {
			healResults = &rs.SyncOpResults{Annotations: annos}
		}
		folderRoles, err = getRoleByFolder(ctx, attr.Session, folderId)
		if err != nil {
			return nil, nil, err
		}
	}
	for _, role := range folderRoles {
		roleID, err := rs.NewResourceID(roleResourceType, role.Id)
		if err != nil {
			return nil, nil, err
		}

		rv = append(rv, grant.NewGrant(resource, collaboratorAccessEntitlement, roleID, grant.WithAnnotation(
			&v2.GrantExpandable{
				EntitlementIds: []string{
					fmt.Sprintf("role:%s:%s", roleID.Resource, collaboratorHasRoleEntitlement),
				},
				Shallow: true,
			},
		)))
	}

	return rv, healResults, nil
}

func newFolderBuilder(client *client.WorkatoClient, disableCustomRolesSync bool) *folderBuilder {
	return &folderBuilder{
		client:                 client,
		cache:                  newCollaboratorCache(client),
		disableCustomRolesSync: disableCustomRolesSync,
	}
}

func folderResource(folder *client.Folder, parentResourceId *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":         folder.Id,
		"name":       folder.Name,
		"create_at":  folder.CreatedAt.String(),
		"parent_id":  folder.ParentId,
		"updated_at": folder.UpdatedAt.String(),
	}

	traits := []rs.AppTraitOption{
		rs.WithAppProfile(profile),
	}

	ret, err := rs.NewAppResource(
		folder.Name,
		folderResourceType,
		folder.Id,
		traits,
		rs.WithParentResourceID(parentResourceId),
		rs.WithAnnotation(
			&v2.ChildResourceType{
				ResourceTypeId: folderResourceType.Id,
			},
		),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func projectFolderResource(project *client.Project, parentResourceId *v2.ResourceId) (*v2.Resource, error) {
	name := fmt.Sprintf("ROOT PROJECT: %s", project.Name)

	profile := map[string]interface{}{
		"id":        project.Id,
		"name":      name,
		"parent_id": nil,
	}

	traits := []rs.AppTraitOption{
		rs.WithAppProfile(profile),
	}

	ret, err := rs.NewAppResource(
		name,
		folderResourceType,
		project.FolderId,
		traits,
		rs.WithParentResourceID(parentResourceId),
		rs.WithAnnotation(
			&v2.ChildResourceType{
				ResourceTypeId: folderResourceType.Id,
			},
		),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
