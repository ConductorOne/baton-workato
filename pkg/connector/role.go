package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
)

var (
	collaboratorHasRoleEntitlement = "collaborator-has"
	roleHasPrivilegeEntitlement    = "privilege-has"
)

type roleBuilder struct {
	client    *client.WorkatoClient
	cache     *collaboratorCache
	roleCache *roleCache
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

		err = o.roleCache.buildCache(ctx)
		if err != nil {
			return nil, "", nil, err
		}
	}

	roles, nextToken, err := o.client.GetRoles(ctx, pToken.Token)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, 0)

	for _, role := range roles {
		us, err := roleResource(&role)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, us)
	}

	// Add base roles
	for _, role := range workato.BaseRoles {
		us, err := workatoBaseRoleResource(&role)
		if err != nil {
			return nil, "", nil, err
		}

		rv = append(rv, us)
	}

	return rv, nextToken, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *roleBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement
	assigmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("%s has Collaborator", resource.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s has %s", resource.DisplayName, collaboratorResourceType.DisplayName)),
	}
	rv = append(rv, entitlement.NewAssignmentEntitlement(resource, collaboratorHasRoleEntitlement, assigmentOptions...))

	assigmentOptions = []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(privilegeResourceType),
		entitlement.WithDescription(fmt.Sprintf("%s has privilege", resource.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s has %s", resource.DisplayName, privilegeResourceType.DisplayName)),
	}
	rv = append(rv, entitlement.NewAssignmentEntitlement(resource, roleHasPrivilegeEntitlement, assigmentOptions...))

	return rv, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *roleBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	// Since roles names are unique, we can use the role name as the key to get all the users that have that role.
	collaborators := o.cache.getUsersByRole(resource.DisplayName)

	rv := make([]*v2.Grant, 0)

	for _, collaborator := range collaborators {
		collaboratorId, err := rs.NewResourceID(collaboratorResourceType, collaborator.User.Id)
		if err != nil {
			return nil, "", nil, err
		}

		for _, roleCollab := range collaborator.User.Roles {
			if roleCollab.RoleName != resource.DisplayName {
				continue
			}

			newGrant := grant.NewGrant(
				resource,
				collaboratorHasRoleEntitlement,
				collaboratorId,
				grant.WithGrantMetadata(map[string]interface{}{
					"environment_type": roleCollab.EnvironmentType,
				}),
			)

			rv = append(rv, newGrant)
		}
	}

	// Base Roles
	if workato.IsBaseRole(resource.DisplayName) {
		role, err := workato.GetBaseRole(resource.DisplayName)
		if err != nil {
			return nil, "", nil, err
		}

		for _, privilege := range role.Privileges {
			privilegeId, err := rs.NewResourceID(privilegeResourceType, privilege.Id())
			if err != nil {
				return nil, "", nil, err
			}

			newGrant := grant.NewGrant(
				resource,
				roleHasPrivilegeEntitlement,
				privilegeId,
				grant.WithAnnotation(&v2.GrantImmutable{}),
			)

			rv = append(rv, newGrant)
		}
	} else {
		role := o.roleCache.getRoleById(resource.Id.Resource)
		if role == nil {
			return rv, "", nil, fmt.Errorf("role %s not found", resource.DisplayName)
		}

		privileges, err := workato.FindRelatedPrivilegesErr(role.Privileges)
		if err != nil {
			return nil, "", nil, err
		}

		for _, privilege := range privileges {
			privilegeId, err := rs.NewResourceID(privilegeResourceType, privilege.Id())
			if err != nil {
				return nil, "", nil, err
			}

			newGrant := grant.NewGrant(
				resource,
				roleHasPrivilegeEntitlement,
				privilegeId,
			)

			rv = append(rv, newGrant)
		}
	}

	return rv, "", nil, nil
}

func newRoleBuilder(client *client.WorkatoClient) *roleBuilder {
	return &roleBuilder{
		client:    client,
		cache:     newCollaboratorCache(client),
		roleCache: newRoleCache(client),
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

func workatoBaseRoleResource(role *workato.Role) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":   role.RoleName,
		"name": role.RoleName,
	}

	traits := []rs.RoleTraitOption{
		rs.WithRoleProfile(profile),
	}

	ret, err := rs.NewRoleResource(
		role.RoleName,
		roleResourceType,
		role.RoleName,
		traits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
