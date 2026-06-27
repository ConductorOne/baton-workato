package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const (
	noAccessRoleName = "No access"
)

var _ connectorbuilder.ResourceSyncerV2 = (*collaboratorBuilder)(nil)

type collaboratorBuilder struct {
	client                 *client.WorkatoClient
	cache                  *collaboratorCache
	env                    workato.Environment
	disableCustomRolesSync bool
}

func (o *collaboratorBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return collaboratorResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *collaboratorBuilder) List(ctx context.Context, _ *v2.ResourceId, attr rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	collaborators, annos, err := o.client.GetCollaborators(ctx)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: annos}, err
	}

	// Set collaborators cache
	if err = o.cache.setCollaboratorsCache(ctx, attr.Session, collaborators); err != nil {
		return nil, &rs.SyncOpResults{Annotations: annos}, err
	}

	rv := make([]*v2.Resource, len(collaborators))

	for i, collaborator := range collaborators {
		us, err := collaboratorResource(collaborator)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: annos}, err
		}
		rv[i] = us
	}

	return rv, &rs.SyncOpResults{Annotations: annos}, nil
}

// Entitlements always returns an empty slice for users.
func (o *collaboratorBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

func (o *collaboratorBuilder) Grants(ctx context.Context, res *v2.Resource, attr rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	rv := make([]*v2.Grant, 0)
	principalID := res.Id

	collaboratorIdStr := res.Id.Resource
	collaboratorId, err := strconv.Atoi(collaboratorIdStr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert collaborator id to int: %w", err)
	}

	collaboratorRoleGrants, err := o.collaboratorRoleGrants(ctx, attr.Session, principalID)
	if err != nil {
		return nil, nil, err
	}

	rv = append(rv, collaboratorRoleGrants...)

	collaboratorRoles, annos, err := o.client.GetCollaboratorPrivileges(ctx, collaboratorId)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, &rs.SyncOpResults{Annotations: annos}, fmt.Errorf("baton-workato: failed to get collaborator privileges: %w", err)
		}
		l.Debug("Collaborator privileges not found, skipping", zap.Int("collaborator_id", collaboratorId))
		return rv, &rs.SyncOpResults{Annotations: annos}, nil
	}

	for _, collaboratorRole := range collaboratorRoles {
		if o.env != workato.All && collaboratorRole.EnvironmentType != o.env.String() {
			l.Debug("Collaborator role environment type does not match, skipping",
				zap.String("environment", collaboratorRole.EnvironmentType),
				zap.String("collaborator_role_name", collaboratorRole.Name),
				zap.String("expected_environment_type", o.env.String()),
			)
			continue
		}

		// Build for privileges
		for group, privileges := range collaboratorRole.Privileges {
			for _, privilege := range privileges {
				newGrant := collaboratorPrivilegeGrant(group, privilege, principalID)
				rv = append(rv, newGrant)
			}
		}

		// Build for folders
		for _, folderId := range collaboratorRole.FolderIDs {
			newGrant := collaboratorFolderGrant(folderId, principalID)
			rv = append(rv, newGrant)
		}
	}

	return rv, &rs.SyncOpResults{Annotations: annos}, nil
}

func collaboratorFolderGrant(folderId int, principalID *v2.ResourceId) *v2.Grant {
	folderResource := &v2.Resource{
		Id: &v2.ResourceId{
			ResourceType: folderResourceType.Id,
			Resource:     strconv.Itoa(folderId),
		},
	}

	// Collaborator only access to the folder if a role have access
	// To update collaborator folder access, the role must be updated
	return grant.NewGrant(
		folderResource,
		collaboratorAccessEntitlement,
		principalID,
		grant.WithAnnotation(&v2.GrantImmutable{}),
	)
}

func collaboratorPrivilegeGrant(group string, privilege string, principalID *v2.ResourceId) *v2.Grant {
	privilegeId := workato.PrivilegeId(group, privilege)

	privilegeResource := &v2.Resource{
		Id: &v2.ResourceId{
			ResourceType: privilegeResourceType.Id,
			Resource:     privilegeId,
		},
	}

	// Collaborator only have privileges if a role is assigned to them
	// To update collaborator privileges, the role must be updated
	return grant.NewGrant(
		privilegeResource,
		assignedEntitlement,
		principalID,
	)
}

func (o *collaboratorBuilder) collaboratorRoleGrants(ctx context.Context, session sessions.SessionStore, principalID *v2.ResourceId) ([]*v2.Grant, error) {
	l := ctxzap.Extract(ctx)

	if principalID.ResourceType != collaboratorResourceType.Id {
		return nil, fmt.Errorf("principal ID is not a collaborator")
	}

	rv := make([]*v2.Grant, 0)
	collaboratorId := principalID.Resource

	collaborator, err := o.cache.getCollaborator(ctx, session, collaboratorId)
	if err != nil {
		return nil, fmt.Errorf("failed to get collaborator from cache: %w", err)
	}

	// Build for roles
	for _, role := range collaborator.Roles {
		if o.env != workato.All && role.EnvironmentType != o.env.String() {
			continue
		}

		if role.RoleName == noAccessRoleName {
			continue
		}

		var roleResource *v2.Resource

		switch role.RoleType {
		case roleTypeEnvironment:
			envRole, err := getEnvironmentRoleByName(ctx, session, role.RoleName)
			if err != nil {
				return nil, err
			}
			if envRole == nil {
				// The list endpoint returns 200 with an empty list when the name does not match.
				envRole, _, err = o.client.GetEnvironmentRoleByName(ctx, role.RoleName)
				if err != nil {
					if isEnvironmentRolesAccessDenied(err) {
						// The API key cannot read environment roles (non-RBAC-v2 workspace
						// or missing privilege). Skip this grant instead of failing the whole
						// collaborator grant sync; the rest of the collaborator's grants
						// (privileges, folders, custom roles) are unaffected.
						l.Warn("baton-workato: cannot access environment roles, skipping environment role grant for collaborator",
							zap.String("role_name", role.RoleName),
							zap.String("environment_type", role.EnvironmentType),
							zap.Error(err),
						)
						continue
					}
					return nil, fmt.Errorf("baton-workato: failed to get environment role %s: %w", role.RoleName, err)
				}
				if envRole == nil {
					l.Debug("baton-workato: environment role not found, it may have been deleted",
						zap.String("role_name", role.RoleName),
					)
					continue
				}
			}
			targetEnv, err := workato.EnvFromString(role.EnvironmentType)
			if err != nil {
				return nil, fmt.Errorf("baton-workato: failed to get target environment from role environment type: %w", err)
			}
			roleResource = &v2.Resource{
				Id: &v2.ResourceId{
					ResourceType: environmentRoleResourceType.Id,
					Resource:     GetRoleResourceID(strconv.Itoa(envRole.Id), targetEnv, o.env),
				},
			}

		case roleTypePrivilegeGroup, "":
			switch {
			case workato.IsBaseRole(role.RoleName):
				baseRole, err := workato.GetBaseRole(role.RoleName)
				if err != nil {
					return nil, fmt.Errorf("baton-workato: failed to get base role: %w", err)
				}
				targetEnv, err := workato.EnvFromString(role.EnvironmentType)
				if err != nil {
					return nil, fmt.Errorf("baton-workato: failed to get target environment from role environment type: %w", err)
				}
				roleResource = &v2.Resource{
					Id: &v2.ResourceId{
						ResourceType: roleResourceType.Id,
						Resource:     GetRoleResourceID(baseRole.RoleName, targetEnv, o.env),
					},
				}
			case !o.disableCustomRolesSync:
				customRole, err := getRoleByName(ctx, session, role.RoleName)
				if err != nil {
					return nil, err
				}
				if customRole == nil {
					l.Debug("baton-workato: custom role not found, it may have been deleted", zap.String("role_name", role.RoleName))
					continue
				}
				targetEnv, err := workato.EnvFromString(role.EnvironmentType)
				if err != nil {
					return nil, fmt.Errorf("baton-workato: failed to get target environment from role environment type: %w", err)
				}
				roleResource = &v2.Resource{
					Id: &v2.ResourceId{
						ResourceType: roleResourceType.Id,
						Resource:     GetRoleResourceID(strconv.Itoa(customRole.Id), targetEnv, o.env),
					},
				}
			default:
				l.Debug("baton-workato: skipping custom role, custom roles sync is disabled", zap.String("role_name", role.RoleName))
				continue
			}

		default:
			l.Debug("baton-workato: unknown role type, skipping", zap.String("role_type", role.RoleType), zap.String("role_name", role.RoleName))
			continue
		}

		newGrant := grant.NewGrant(
			roleResource,
			collaboratorHasRoleEntitlement,
			principalID,
			grant.WithGrantMetadata(map[string]any{
				"environment_type": role.EnvironmentType,
			}),
		)
		rv = append(rv, newGrant)
	}
	return rv, nil
}

func newCollaboratorBuilder(client *client.WorkatoClient, env workato.Environment, disableCustomRolesSync bool) *collaboratorBuilder {
	return &collaboratorBuilder{
		client:                 client,
		cache:                  newCollaboratorCache(client),
		env:                    env,
		disableCustomRolesSync: disableCustomRolesSync,
	}
}

func collaboratorResource(collaborator *client.Collaborator) (*v2.Resource, error) {
	var userStatus = v2.UserTrait_Status_STATUS_ENABLED

	profile := map[string]any{
		"id":         collaborator.Id,
		"email":      collaborator.Email,
		"name":       collaborator.Name,
		"externalId": collaborator.ExternalId,
		"createdAt":  collaborator.CreatedAt.String(),
		"grantType":  collaborator.GrantType,
		"timeZone":   collaborator.TimeZone,
	}

	traits := []rs.UserTraitOption{
		rs.WithUserProfile(profile),
		rs.WithStatus(userStatus),
		rs.WithEmail(collaborator.Email, true),
		rs.WithUserLogin(collaborator.Email),
		rs.WithCreatedAt(collaborator.CreatedAt),
		rs.WithAccountType(v2.UserTrait_ACCOUNT_TYPE_HUMAN),
	}

	ret, err := rs.NewUserResource(
		collaborator.Name,
		collaboratorResourceType,
		collaborator.Id,
		traits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
