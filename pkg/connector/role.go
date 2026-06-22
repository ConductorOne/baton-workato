package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	collaboratorHasRoleEntitlement = "collaborator-has"
)

var _ connectorbuilder.ResourceSyncerV2 = (*roleBuilder)(nil)
var _ connectorbuilder.ResourceProvisionerV2Limited = (*roleBuilder)(nil)

type roleBuilder struct {
	client                 *client.WorkatoClient
	cache                  *collaboratorCache
	env                    workato.Environment
	disableCustomRolesSync bool
}

func (o *roleBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return roleResourceType
}

// List returns all the Workato base roles and custom roles.
func (o *roleBuilder) List(ctx context.Context, _ *v2.ResourceId, attr rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	rv := make([]*v2.Resource, 0)
	envs := resolveEnvironments(o.env)

	if !o.disableCustomRolesSync {
		roles, nextToken, annos, err := o.client.GetRoles(ctx, attr.PageToken.Token)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: annos}, err
		}

		if err = setRolesCache(ctx, attr.Session, roles); err != nil {
			return nil, &rs.SyncOpResults{Annotations: annos}, err
		}

		for _, targetEnv := range envs {
			for _, role := range roles {
				us, err := roleResource(role, o.env, targetEnv)
				if err != nil {
					return nil, &rs.SyncOpResults{Annotations: annos}, err
				}
				rv = append(rv, us)
			}
		}

		if nextToken != "" {
			return rv, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil
		}

		// Last page of custom roles: append base roles and return with rate limit propagated
		for _, targetEnv := range envs {
			for _, role := range workato.BaseRoles {
				us, err := workatoBaseRoleResource(&role, o.env, targetEnv)
				if err != nil {
					return nil, &rs.SyncOpResults{Annotations: annos}, err
				}
				rv = append(rv, us)
			}
		}
		return rv, &rs.SyncOpResults{Annotations: annos}, nil
	}

	// Custom roles sync disabled: only base roles, no API call
	for _, targetEnv := range envs {
		for _, role := range workato.BaseRoles {
			us, err := workatoBaseRoleResource(&role, o.env, targetEnv)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, us)
		}
	}
	return rv, nil, nil
}

// Entitlements returns an entitlement for the role to be assigned to a collaborator.
func (o *roleBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	var rv []*v2.Entitlement
	assigmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("%s has Collaborator", resource.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s has %s", resource.DisplayName, collaboratorResourceType.DisplayName)),
	}
	rv = append(rv, entitlement.NewAssignmentEntitlement(resource, collaboratorHasRoleEntitlement, assigmentOptions...))

	return rv, nil, nil
}

// Grants returns the privileges granted to a role.
func (o *roleBuilder) Grants(ctx context.Context, resource *v2.Resource, attr rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	rv := make([]*v2.Grant, 0)

	roleTrait, err := rs.GetRoleTrait(resource)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get role trait: %w", err)
	}
	profile := roleTrait.GetProfile()
	if profile == nil {
		return nil, nil, fmt.Errorf("role profile not found on resource %s", resource.Id.Resource)
	}
	roleId, ok := profile.AsMap()["id"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("role id not found on resource %s", resource.Id.Resource)
	}
	roleName, ok := profile.AsMap()["name"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("role name not found on resource %s", resource.Id.Resource)
	}

	// Base Roles - privilege grants implementation
	if workato.IsBaseRole(roleName) {
		role, err := workato.GetBaseRole(roleName)
		if err != nil {
			return nil, nil, err
		}

		for _, privilege := range role.Privileges {
			privilegeId, err := rs.NewResourceID(privilegeResourceType, privilege.Id())
			if err != nil {
				return nil, nil, err
			}

			newGrant := grant.NewGrant(
				&v2.Resource{
					Id: privilegeId,
				},
				assignedEntitlement,
				resource.Id,
				grant.WithAnnotation(
					&v2.GrantImmutable{},
					&v2.GrantExpandable{
						EntitlementIds: []string{
							fmt.Sprintf("role:%s:%s", resource.Id.Resource, collaboratorHasRoleEntitlement),
						},
						Shallow: true,
					},
				),
			)

			rv = append(rv, newGrant)
		}
		return rv, nil, nil
	}

	if !o.disableCustomRolesSync {
		// privilege grants implementation
		role := getRoleById(ctx, attr.Session, roleId)
		if role == nil {
			return rv, nil, uhttp.WrapErrors(codes.NotFound, fmt.Sprintf("role %s (%s) not found", resource.DisplayName, roleId))
		}

		privileges, err := workato.FindRelatedPrivilegesErr(role.Privileges)
		if err != nil {
			return nil, nil, err
		}

		for _, privilege := range privileges {
			privilegeId, err := rs.NewResourceID(privilegeResourceType, privilege.Id())
			if err != nil {
				return nil, nil, err
			}

			newGrant := grant.NewGrant(
				&v2.Resource{
					Id: privilegeId,
				},
				assignedEntitlement,
				resource.Id,
				grant.WithAnnotation(
					&v2.GrantExpandable{
						EntitlementIds: []string{
							fmt.Sprintf("role:%s:%s", resource.Id.Resource, collaboratorHasRoleEntitlement),
						},
						Shallow: true,
					},
				),
			)

			rv = append(rv, newGrant)
		}
	}

	return rv, nil, nil
}

func (o *roleBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	if principal.Id.ResourceType != collaboratorResourceType.Id {
		return nil, nil, fmt.Errorf("grant not implemented for %s", principal.Id.ResourceType)
	}

	// Grant a role to a collaborator
	userID, err := strconv.Atoi(principal.Id.Resource)
	if err != nil {
		return nil, nil, err
	}

	roleTrait, err := rs.GetRoleTrait(entitlement.Resource)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-workato: failed to get role trait: %w", err)
	}
	profile := roleTrait.GetProfile()
	if profile == nil {
		return nil, nil, fmt.Errorf("baton-workato: role profile is nil on resource %s", entitlement.Resource.Id.Resource)
	}
	roleName, ok := profile.AsMap()["name"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("baton-workato: role name missing from profile on resource %s", entitlement.Resource.Id.Resource)
	}

	_, envType, err := parseRoleResourceID(entitlement.Resource.Id.Resource, o.env)
	if err != nil {
		return nil, nil, err
	}

	roles := []client.SimpleRole{{
		RoleName:        roleName,
		EnvironmentType: envType.String(),
	}}

	annos, err := o.client.UpdateCollaboratorRoles(ctx, userID, roles)
	if err != nil {
		return nil, annos, err
	}

	newGrant := grant.NewGrant(
		entitlement.Resource,
		collaboratorHasRoleEntitlement,
		principal.Id,
		grant.WithGrantMetadata(map[string]interface{}{
			"environment_type": envType.String(),
		}),
	)
	return []*v2.Grant{newGrant}, annos, nil
}

func (o *roleBuilder) Revoke(_ context.Context, _ *v2.Grant) (annotations.Annotations, error) {
	return nil, status.Errorf(codes.Unimplemented, "baton-workato: revoke is not supported for legacy roles")
}

func newRoleBuilder(client *client.WorkatoClient, env workato.Environment, disableCustomRolesSync bool) *roleBuilder {
	return &roleBuilder{
		client:                 client,
		cache:                  newCollaboratorCache(client),
		env:                    env,
		disableCustomRolesSync: disableCustomRolesSync,
	}
}

func roleResource(role *client.Role, envConfig workato.Environment, targetEnv workato.Environment) (*v2.Resource, error) {
	if targetEnv == workato.All {
		return nil, fmt.Errorf("target environment %s is not supported for role resources", targetEnv.String())
	}

	id := strconv.Itoa(role.Id)

	profile := map[string]interface{}{
		"id":          id,
		"name":        role.Name,
		"environment": targetEnv.String(),
		"create_at":   role.CreatedAt.String(),
		"inheritable": role.Inheritable,
		"updated_at":  role.UpdatedAt.String(),
	}

	traits := []rs.RoleTraitOption{
		rs.WithRoleProfile(profile),
	}

	ret, err := rs.NewRoleResource(
		fmt.Sprintf("%s (%s)", role.Name, targetEnv.String()),
		roleResourceType,
		GetRoleResourceID(id, targetEnv, envConfig),
		traits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// workatoBaseRoleResource creates a new role resource for a base role.
// envConfig is the environment configured for the connector.
// targetEnv is the environment to create the role resource for.
func workatoBaseRoleResource(role *workato.Role, envConfig workato.Environment, targetEnv workato.Environment) (*v2.Resource, error) {
	if targetEnv == workato.All {
		return nil, fmt.Errorf("target environment %s is not supported for base roles", targetEnv.String())
	}

	profile := map[string]interface{}{
		"id":          role.RoleName,
		"name":        role.RoleName,
		"environment": targetEnv.String(),
	}

	traits := []rs.RoleTraitOption{
		rs.WithRoleProfile(profile),
	}

	ret, err := rs.NewRoleResource(
		fmt.Sprintf("%s (%s)", role.RoleName, targetEnv.String()),
		roleResourceType,
		GetRoleResourceID(role.RoleName, targetEnv, envConfig),
		traits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
