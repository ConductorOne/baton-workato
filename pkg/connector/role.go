package connector

import (
	"context"
	"fmt"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

var (
	collaboratorHasRoleEntitlement = "has"
)

type roleBuilder struct {
	client *client.WorkatoClient
	cache  *collaboratorCache
}

func (o *roleBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return roleResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *roleBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {

	if pToken.Token == "" {
		err := o.cache.buildCache(ctx)
		if err != nil {
			return nil, "", nil, err
		}
	}

	roles, nextToken, err := o.client.GetRoles(ctx, pToken.Token)
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
	var rv []*v2.Entitlement
	assigmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("Role has user %s", resource.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s has %s", resource.DisplayName, collaboratorResourceType.DisplayName)),
	}
	rv = append(rv, entitlement.NewAssignmentEntitlement(resource, collaboratorHasRoleEntitlement, assigmentOptions...))

	return rv, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *roleBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	// Since roles names are unique, we can use the role name as the key to get all the users that have that role.
	collaborators := o.cache.getUsersByRole(resource.DisplayName)

	rv := make([]*v2.Grant, len(collaborators))

	for i, collaborator := range collaborators {
		collaboratorId, err := rs.NewResourceID(collaboratorResourceType, collaborator.User.Id)
		if err != nil {
			return nil, "", nil, err
		}

		newGrant := grant.NewGrant(
			resource,
			collaboratorHasRoleEntitlement,
			collaboratorId,
		)

		rv[i] = newGrant
	}

	return rv, "", nil, nil
}

func newRoleBuilder(client *client.WorkatoClient) *roleBuilder {
	return &roleBuilder{
		client: client,
		cache:  newCollaboratorCache(client),
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

	traits := []rs.RoleTraitOption{
		rs.WithRoleProfile(profile),
	}

	ret, err := rs.NewRoleResource(
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
