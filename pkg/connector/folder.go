package connector

import (
	"context"

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
	folders, nextToken, err := o.client.GetFolders(ctx, nil, pToken)
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
	}

	if len(folders) == 0 {
		nextToken = ""
	}

	return rv, nextToken, nil, nil
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
