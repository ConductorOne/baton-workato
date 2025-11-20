package connector

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
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

type roleBuilder struct {
	client                 *client.WorkatoClient
	env                    workato.Environment
	disableCustomRolesSync bool
}

func (o *roleBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return roleResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *roleBuilder) List(ctx context.Context, _ *v2.ResourceId, attr rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("Listing roles")

	rv := make([]*v2.Resource, 0)

	var nextToken string

	if !o.disableCustomRolesSync {
		var roles []client.Role
		var err error
		roles, nextToken, err = o.client.GetRoles(ctx, attr.PageToken.Token)
		if err != nil {
			return nil, nil, err
		}

		//cache roles
		setRolesCache(ctx, attr.Session, roles)

		for _, role := range roles {
			us, err := roleResource(&role)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, us)
		}
	}

	// Add base roles
	for _, role := range workato.BaseRoles {
		us, err := workatoBaseRoleResource(&role)
		if err != nil {
			return nil, nil, err
		}

		rv = append(rv, us)
	}

	return rv, &rs.SyncOpResults{
		NextPageToken: nextToken,
	}, nil
}

// Entitlements always returns an empty slice for users.
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

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *roleBuilder) Grants(ctx context.Context, resource *v2.Resource, attr rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)

	// Since roles names are unique, we can use the role name as the key to get all the users that have that role.
	collaborators := getUsersByRole(ctx, attr.Session, resource.DisplayName)

	rv := make([]*v2.Grant, 0)

	for _, collaborator := range collaborators {
		collaboratorId, err := rs.NewResourceID(collaboratorResourceType, collaborator.User.Id)
		if err != nil {
			return nil, nil, err
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
	} else if !o.disableCustomRolesSync {
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

		roleName := entitlement.Resource.Id.Resource
		userID, err := strconv.Atoi(resource.Id.Resource)
		if err != nil {
			return nil, nil, err
		}

		collaborator, err := o.client.GetCollaboratorPrivileges(ctx, userID)
		if err != nil {
			return nil, nil, err
		}

		roles := toSimpleRole(collaborator)

		newRole := client.SimpleRole{
			RoleName:        roleName,
			EnvironmentType: o.env.String(),
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

	return nil, nil, fmt.Errorf("baton-workato grant not implemented for resource type %s", resource.Id.ResourceType)
}

func (o *roleBuilder) Revoke(_ context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	return nil, fmt.Errorf("baton-workato revoke not implemented for resource type %s", grant.Principal.Id.ResourceType)
}

func newRoleBuilder(client *client.WorkatoClient, env workato.Environment, disableCustomRolesSync bool) *roleBuilder {
	return &roleBuilder{
		client:                 client,
		env:                    env,
		disableCustomRolesSync: disableCustomRolesSync,
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

func toSimpleRole(collaboratorRoles []*client.CollaboratorPrivilege) []client.SimpleRole {
	roles := make([]client.SimpleRole, 0)
	for _, role := range collaboratorRoles {
		roles = append(roles, role.SimpleRole())
	}

	return roles
}
