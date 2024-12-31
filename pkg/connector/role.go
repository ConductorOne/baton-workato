package connector

import (
	"context"

	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

type roleBuilder struct {
	client *client.WorkatoClient
}

func (o *roleBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return roleResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *roleBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	roles, nextToken, err := o.client.GetRoles(ctx, pToken)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, len(roles))

	for i, role := range roles {
		us, err := roleResource(&role)
		if err != nil {
			return nil, "", nil, err
		}
		rv[i] = us
	}

	if len(roles) == 0 {
		nextToken = ""
	}

	return rv, nextToken, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *roleBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *roleBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newRoleBuilder(client *client.WorkatoClient) *roleBuilder {
	return &roleBuilder{
		client: client,
	}
}

func roleResource(role *client.Role) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":          role.Id,
		"name":        role.Name,
		"create_at":   role.CreatedAt.String(),
		"inheritable": role.Inheritable,
		"updated_at":  role.UpdatedAt.String(),
	}

	traits := []resource.RoleTraitOption{
		resource.WithRoleProfile(profile),
	}

	ret, err := resource.NewRoleResource(
		role.Name,
		roleResourceType,
		role.Id,
		traits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
