package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"

	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/cpagination"
	"github.com/conductorone/baton-workato/pkg/connector/workato"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

const (
	collaboratorAccessEntitlement = "collaborator-access"
)

type folderBuilder struct {
	client                 *client.WorkatoClient
	cache                  *collaboratorCache
	disableCustomRolesSync bool
}

func (o *folderBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return folderResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *folderBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, attr rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("Listing folders")

	rv := make([]*v2.Resource, 0)

	if parentResourceID == nil {
		return nil, nil, nil
	}

	if parentResourceID.ResourceType == projectResourceType.Id {
		projects, nextToken, err := o.client.GetProjects(ctx, attr.PageToken.Token)
		if err != nil {
			return nil, nil, err
		}

		for _, project := range projects {
			// Create a resource for the project
			projectRs, err := projectFolderResource(&project, parentResourceID)
			if err != nil {
				return nil, nil, err
			}

			rv = append(rv, projectRs)
		}

		return rv, &rs.SyncOpResults{
			NextPageToken: nextToken,
		}, nil
	}

	if parentResourceID.ResourceType == folderResourceType.Id {
		parentId, err := strconv.Atoi(parentResourceID.Resource)
		if err != nil {
			return nil, nil, err
		}

		folders, nextToken, err := o.client.GetFolders(ctx, &parentId, attr.PageToken.Token)
		if err != nil {
			return nil, nil, err
		}

		for _, folder := range folders {
			us, err := folderResource(&folder, parentResourceID)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, us)
		}

		return rv, &rs.SyncOpResults{
			NextPageToken: nextToken,
		}, nil
	}

	l.Warn("Unknown parent resource type", zap.String("parent_resource_type", parentResourceID.ResourceType))
	return nil, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *folderBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	var rv []*v2.Entitlement

	assigmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("%s can acess %s", collaboratorResourceType.DisplayName, resource.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s acess %s", collaboratorResourceType.DisplayName, resource.DisplayName)),
	}
	rv = append(rv, entitlement.NewPermissionEntitlement(resource, collaboratorAccessEntitlement, assigmentOptions...))

	return rv, nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *folderBuilder) Grants(ctx context.Context, resource *v2.Resource, attr rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	type Bag struct {
		ResourceTypeID string
		Page           int
	}

	bag, err := cpagination.GenBagFromToken[Bag](attr.PageToken)
	if err != nil {
		return nil, nil, err
	}

	if bag.Current() == nil {
		bag.Push(Bag{
			ResourceTypeID: roleResourceType.Id,
			Page:           0,
		})

		nextToken, err := bag.Marshal()
		if err != nil {
			return nil, nil, err
		}

		return nil, &rs.SyncOpResults{
			NextPageToken: nextToken,
		}, nil
	}

	state := bag.Pop()

	var rv []*v2.Grant

	if state.ResourceTypeID == roleResourceType.Id && !o.disableCustomRolesSync {
		folderId := resource.Id.Resource
		roles := getRoleByFolder(ctx, attr.Session, folderId)

		for _, role := range roles {
			roleID, err := rs.NewResourceID(roleResourceType, role.Id)
			if err != nil {
				return nil, nil, err
			}

			newGrant := grant.NewGrant(resource, collaboratorAccessEntitlement, roleID, grant.WithAnnotation(
				&v2.GrantExpandable{
					EntitlementIds: []string{
						fmt.Sprintf("role:%s:%s", roleID.Resource, collaboratorHasRoleEntitlement),
					},
					Shallow: true,
				},
			))
			rv = append(rv, newGrant)
		}
	}

	nextToken, err := bag.Marshal()
	if err != nil {
		return nil, nil, err
	}

	return rv, &rs.SyncOpResults{
		NextPageToken: nextToken,
	}, nil
}

func newFolderBuilder(client *client.WorkatoClient, env workato.Environment, disableCustomRolesSync bool) *folderBuilder {
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
