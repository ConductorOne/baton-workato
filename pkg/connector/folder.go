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

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

const (
	collaboratorAccessEntitlement = "collaborator-access"
	roleAccessEntitlement         = "role-access"
)

type folderBuilder struct {
	client    *client.WorkatoClient
	cache     *collaboratorCache
	roleCache *roleCache
}

func (o *folderBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return folderResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *folderBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	// Init cache
	if pToken.Token == "" && parentResourceID == nil {
		err := o.cache.buildCache(ctx)
		if err != nil {
			return nil, "", nil, err
		}

		err = o.roleCache.buildCache(ctx)
		if err != nil {
			return nil, "", nil, err
		}
	}

	rv := make([]*v2.Resource, 0)

	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	if parentResourceID.ResourceType == projectResourceType.Id {
		projects, nextToken, err := o.client.GetProjects(ctx, pToken.Token)
		if err != nil {
			return nil, "", nil, err
		}

		for _, project := range projects {
			// Create a resource for the project
			projectRs, err := projectFolderResource(&project, parentResourceID)
			if err != nil {
				return nil, "", nil, err
			}

			rv = append(rv, projectRs)
		}

		return rv, nextToken, nil, err
	}

	if parentResourceID.ResourceType == folderResourceType.Id {
		parentId, err := strconv.Atoi(parentResourceID.Resource)
		if err != nil {
			return nil, "", nil, err
		}

		folders, nextToken, err := o.client.GetFolders(ctx, &parentId, pToken.Token)
		if err != nil {
			return nil, "", nil, err
		}

		for _, folder := range folders {
			us, err := folderResource(&folder, parentResourceID)
			if err != nil {
				return nil, "", nil, err
			}
			rv = append(rv, us)
		}

		return rv, nextToken, nil, nil
	}

	l.Warn("Unknown parent resource type", zap.String("parent_resource_type", parentResourceID.ResourceType))
	return nil, "", nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *folderBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement
	assigmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(roleResourceType),
		entitlement.WithDescription(fmt.Sprintf("%s can acess %s", roleResourceType.DisplayName, resource.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s acess %s", roleResourceType.DisplayName, resource.DisplayName)),
	}
	rv = append(rv, entitlement.NewPermissionEntitlement(resource, roleAccessEntitlement, assigmentOptions...))

	assigmentOptions = []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("%s can acess %s", collaboratorResourceType.DisplayName, resource.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s acess %s", collaboratorResourceType.DisplayName, resource.DisplayName)),
	}
	rv = append(rv, entitlement.NewPermissionEntitlement(resource, collaboratorAccessEntitlement, assigmentOptions...))

	return rv, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *folderBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	type Bag struct {
		ResourceTypeID string
		Page           int
	}

	bag, err := cpagination.GenBagFromToken[Bag](*pToken)
	if err != nil {
		return nil, "", nil, err
	}

	if bag.Current() == nil {
		bag.Push(Bag{
			ResourceTypeID: collaboratorResourceType.Id,
			Page:           0,
		})

		bag.Push(Bag{
			ResourceTypeID: roleResourceType.Id,
			Page:           0,
		})

		nextToken, err := bag.Marshal()
		if err != nil {
			return nil, "", nil, err
		}

		return nil, nextToken, nil, nil
	}

	state := bag.Pop()

	var rv []*v2.Grant

	if state.ResourceTypeID == collaboratorResourceType.Id {
		folderId, err := strconv.Atoi(resource.Id.Resource)
		if err != nil {
			return nil, "", nil, err
		}

		collaborators := o.cache.getUsersByFolder(folderId)

		for _, collaborator := range collaborators {
			collaboratorId, err := rs.NewResourceID(collaboratorResourceType, collaborator.User.Id)
			if err != nil {
				return nil, "", nil, err
			}

			// Collaborator only access to the folder if a role have access
			// To update collaborator folder access, the role must be updated
			newGrant := grant.NewGrant(
				resource,
				collaboratorAccessEntitlement,
				collaboratorId,
				grant.WithAnnotation(&v2.GrantImmutable{}),
			)
			rv = append(rv, newGrant)
		}
	}

	if state.ResourceTypeID == roleResourceType.Id {
		folderId, err := strconv.Atoi(resource.Id.Resource)
		if err != nil {
			return nil, "", nil, err
		}

		roles := o.roleCache.getRoleByFolder(folderId)

		for _, role := range roles {
			roleID, err := rs.NewResourceID(roleResourceType, role.Id)
			if err != nil {
				return nil, "", nil, err
			}

			newGrant := grant.NewGrant(resource, roleAccessEntitlement, roleID)
			rv = append(rv, newGrant)
		}
	}

	nextToken, err := bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return rv, nextToken, nil, nil
}

func newFolderBuilder(client *client.WorkatoClient) *folderBuilder {
	return &folderBuilder{
		client:    client,
		cache:     newCollaboratorCache(client),
		roleCache: newRoleCache(client),
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
