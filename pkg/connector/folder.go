package connector

import (
	"context"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

type folderBuilder struct {
	client *client.WorkatoClient
}

func (o *folderBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return folderResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *folderBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var bag pagination.Bag

	var parentId *int
	var token string

	if pToken.Token != "" {
		err := bag.Unmarshal(pToken.Token)
		if err != nil {
			return nil, "", nil, err
		}

		state := bag.Pop()

		if state.ResourceID != "" {
			parentIdInt, err := strconv.Atoi(state.ResourceID)
			if err != nil {
				return nil, "", nil, err
			}

			// Parent Folder ID
			parentId = &parentIdInt
		}

		// Page Number
		token = state.Token
	}

	folders, nextToken, err := o.client.GetFolders(ctx, parentId, token)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, len(folders))

	for i, folder := range folders {
		us, err := folderResource(&folder)
		if err != nil {
			return nil, "", nil, err
		}
		rv[i] = us

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

	marshal, err := bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return rv, marshal, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *folderBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *folderBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newFolderBuilder(client *client.WorkatoClient) *folderBuilder {
	return &folderBuilder{
		client: client,
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

	userTraits := []resource.AppTraitOption{
		resource.WithAppProfile(profile),
	}

	ret, err := resource.NewAppResource(
		folder.Name,
		folderResourceType,
		folder.Id,
		userTraits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
