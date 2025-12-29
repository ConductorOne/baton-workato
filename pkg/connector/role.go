package connector

import (
	"context"
	"fmt"
	"slices"
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
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
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
	l := ctxzap.Extract(ctx)
	l.Debug("Listing roles")

	rv := make([]*v2.Resource, 0)

	var nextToken string

	var envs []workato.Environment
	if o.env == workato.All {
		envs = workato.AllEnvironments()
	} else {
		envs = append(envs, o.env)
	}

	if !o.disableCustomRolesSync {
		var roles []client.Role
		var err error
		roles, nextToken, err = o.client.GetRoles(ctx, attr.PageToken.Token)
		if err != nil {
			return nil, nil, err
		}

		// cache roles
		err = setRolesCache(ctx, attr.Session, roles)
		if err != nil {
			return nil, nil, err
		}

		for _, targetEnv := range envs {
			for _, role := range roles {
				us, err := roleResource(&role, o.env, targetEnv)
				if err != nil {
					return nil, nil, err
				}
				rv = append(rv, us)
			}
		}
	}

	if nextToken == "" {
		// Add base roles
		for _, targetEnv := range envs {
			for _, role := range workato.BaseRoles {
				us, err := workatoBaseRoleResource(&role, o.env, targetEnv)
				if err != nil {
					return nil, nil, err
				}
				rv = append(rv, us)
			}
		}
	}

	return rv, &rs.SyncOpResults{
		NextPageToken: nextToken,
	}, nil
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
	l := ctxzap.Extract(ctx)
	rv := make([]*v2.Grant, 0)

	// Base Roles - privilege grants implementation
	if workato.IsBaseRole(resource.DisplayName) {
		role, err := workato.GetBaseRole(resource.DisplayName)
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
		role := getRoleById(ctx, attr.Session, resource.Id.Resource)
		if role == nil {
			l.Warn("role not found", zap.String("role_name", resource.DisplayName), zap.String("role_id", resource.Id.Resource))
			return rv, nil, uhttp.WrapErrors(codes.NotFound, fmt.Sprintf("role %s (%s) not found", resource.DisplayName, resource.Id.Resource))
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

func (o *roleBuilder) Grant(ctx context.Context, resource *v2.Resource, entitlement *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	// Grant a role to a collaborator
	if resource.Id.ResourceType == collaboratorResourceType.Id {
		grants := make([]*v2.Grant, 0)

		userID, err := strconv.Atoi(resource.Id.Resource)
		if err != nil {
			return nil, nil, err
		}

		collaborator, err := o.client.GetCollaboratorPrivileges(ctx, userID)
		if err != nil {
			return nil, nil, err
		}

		roles := toSimpleRole(collaborator)

		roleTrait, err := rs.GetRoleTrait(entitlement.Resource)
		if err != nil {
			return nil, nil, err
		}
		profile := roleTrait.GetProfile()
		if profile == nil {
			return nil, nil, fmt.Errorf("role profile not found")
		}
		roleName, ok := profile.AsMap()["role_name"].(string)
		if !ok {
			return nil, nil, fmt.Errorf("role name is missing or invalid")
		}
		environmentType, ok := profile.AsMap()["environment"].(string)
		if !ok {
			return nil, nil, fmt.Errorf("environment value is missing or invalid")
		}

		newRole := client.SimpleRole{
			RoleName:        roleName,
			EnvironmentType: environmentType,
		}

		index := slices.IndexFunc(roles, func(other client.SimpleRole) bool {
			return other.Equals(newRole)
		})

		if index >= 0 {
			return []*v2.Grant{}, annotations.New(&v2.GrantAlreadyExists{}), nil
		}

		// Workato just accept one role per environment
		sameEnvIndex := slices.IndexFunc(roles, func(other client.SimpleRole) bool {
			return other.EnvironmentType == o.env.String()
		})

		if sameEnvIndex >= 0 {
			roles[sameEnvIndex] = newRole
		} else {
			roles = append(roles, newRole)
		}

		err = o.client.UpdateCollaboratorRoles(ctx, userID, roles)
		if err != nil {
			return nil, nil, err
		}

		collaboratorId, err := rs.NewResourceID(collaboratorResourceType, userID)
		if err != nil {
			return nil, nil, err
		}

		newGrant := grant.NewGrant(
			resource,
			collaboratorHasRoleEntitlement,
			collaboratorId,
			grant.WithGrantMetadata(map[string]interface{}{
				"environment_type": o.env.String(),
			}),
		)

		grants = append(grants, newGrant)

		return grants, nil, nil
	}

	return nil, nil, fmt.Errorf("grant not implemented for %s", resource.Id.ResourceType)
}

func (o *roleBuilder) Revoke(_ context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	return nil, fmt.Errorf("revoke not implemented for %s", grant.Principal.Id.ResourceType)
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
	name := role.Name

	// For backward compatibility, do not change the role IDs if the environment is set to a specific environment.
	if envConfig == workato.All {
		id = workato.RoleResourceID(id, targetEnv.String())
		name = fmt.Sprintf("%s (%s)", role.Name, targetEnv.String())
	}

	profile := map[string]interface{}{
		"id":          id,
		"name":        name,
		"role_name":   role.Name,
		"environment": targetEnv.String(),
		"create_at":   role.CreatedAt.String(),
		"inheritable": role.Inheritable,
		"updated_at":  role.UpdatedAt.String(),
	}

	traits := []rs.RoleTraitOption{
		rs.WithRoleProfile(profile),
	}

	ret, err := rs.NewRoleResource(
		name,
		roleResourceType,
		id,
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

	id := role.RoleName
	name := role.RoleName

	// For backward compatibility, do not change the role IDs if the environment is set to a specific environment.
	if envConfig == workato.All {
		id = workato.RoleResourceID(role.RoleName, targetEnv.String())
		name = fmt.Sprintf("%s (%s)", role.RoleName, targetEnv.String())
	}

	profile := map[string]interface{}{
		"id":          id,
		"name":        name,
		"role_name":   role.RoleName,
		"environment": targetEnv.String(),
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

func toSimpleRole(collaboratorRoles []*client.CollaboratorPrivilege) []client.SimpleRole {
	roles := make([]client.SimpleRole, 0)
	for _, role := range collaboratorRoles {
		roles = append(roles, role.SimpleRole())
	}

	return roles
}
