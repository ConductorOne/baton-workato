package connector

import (
	"context"
	"fmt"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-workato/pkg/connector/cpagination"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"strconv"

	"github.com/conductorone/baton-workato/pkg/connector/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

const (
	accessEntitlement = "access"
)

type folderBuilder struct {
	client *client.WorkatoClient
	cache  *collaboratorCache
}

func (o *folderBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return folderResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *folderBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	var bag pagination.Bag

	if pToken.Token == "" {
		err := o.cache.buildCache(ctx)
		if err != nil {
			return nil, "", nil, err
		}

		// List all projects to get root folders
		bag.Push(pagination.PageState{
			Token:          "",
			ResourceTypeID: projectResourceType.Id,
			ResourceID:     "",
		})
	} else {
		err := bag.Unmarshal(pToken.Token)
		if err != nil {
			return nil, "", nil, err
		}
	}

	rv := make([]*v2.Resource, 0)
	state := bag.Pop()

	if state == nil {
		return nil, "", nil, nil
	}

	// Fetch all projects folders
	if state.ResourceTypeID == projectResourceType.Id {
		projects, nextToken, err := o.client.GetProjects(ctx, state.Token)
		if err != nil {
			return nil, "", nil, err
		}

		if len(projects) != 0 {
			bag.Push(pagination.PageState{
				Token:          nextToken,
				ResourceTypeID: projectResourceType.Id,
				ResourceID:     "",
			})
		}

		for _, project := range projects {
			l.Info("Project", zap.String("name", project.Name))

			// Create a resource for the project
			projectRs, err := folderFolderResource(&project)
			if err != nil {
				return nil, "", nil, err
			}

			rv = append(rv, projectRs)

			// Fetch all folders for the project
			bag.Push(pagination.PageState{
				Token:          "",
				ResourceTypeID: folderResourceType.Id,
				ResourceID:     strconv.Itoa(project.FolderId),
			})
		}
	}

	// Fetch Folder recursively
	if state.ResourceTypeID == folderResourceType.Id {
		var parentId *int

		if state.ResourceID != "" {
			parentIdInt, err := strconv.Atoi(state.ResourceID)
			if err != nil {
				return nil, "", nil, err
			}

			// Parent Folder ID
			parentId = &parentIdInt
		}

		folders, nextToken, err := o.client.GetFolders(ctx, parentId, state.Token)
		if err != nil {
			return nil, "", nil, err
		}

		for _, folder := range folders {
			us, err := folderResource(&folder)
			if err != nil {
				return nil, "", nil, err
			}
			rv = append(rv, us)

			bag.Push(pagination.PageState{
				Token:          "",
				ResourceTypeID: folderResourceType.Id,
				ResourceID:     us.Id.Resource,
			})
		}

		if len(folders) != 0 {
			nextStateId := ""

			if parentId != nil {
				nextStateId = strconv.Itoa(*parentId)
			}

			bag.Push(pagination.PageState{
				Token:          nextToken,
				ResourceTypeID: folderResourceType.Id,
				ResourceID:     nextStateId,
			})
		}
	}

	marshal, err := bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return rv, marshal, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *folderBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {

	var rv []*v2.Entitlement
	assigmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(roleResourceType),
		entitlement.WithDescription(fmt.Sprintf("role can acess the folder")),
		entitlement.WithDisplayName(fmt.Sprintf("%s acess %s", roleResourceType.DisplayName, resource.DisplayName)),
	}
	rv = append(rv, entitlement.NewAssignmentEntitlement(resource, accessEntitlement, assigmentOptions...))

	assigmentOptions = []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("Collaborator can acess the folder")),
		entitlement.WithDisplayName(fmt.Sprintf("%s acess %s", collaboratorResourceType.DisplayName, resource.DisplayName)),
	}
	rv = append(rv, entitlement.NewAssignmentEntitlement(resource, accessEntitlement, assigmentOptions...))

	return rv, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *folderBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

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

		l.Info("Collaborators CACHE", zap.Int("collaborators", len(collaborators)), zap.Int("folderId", folderId))

		for _, collaborator := range collaborators {

			collaboratorId, err := rs.NewResourceID(collaboratorResourceType, collaborator.User.Id)
			if err != nil {
				return nil, "", nil, err
			}

			newGrant := grant.NewGrant(resource, accessEntitlement, collaboratorId)
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
		client: client,
		cache:  newPrivilegeCache(client),
	}
}

func folderResource(folder *client.Folder) (*v2.Resource, error) {
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
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func folderFolderResource(project *client.Project) (*v2.Resource, error) {
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
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
