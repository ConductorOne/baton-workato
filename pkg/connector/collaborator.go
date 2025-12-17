package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

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
func (o *collaboratorBuilder) List(ctx context.Context, _ *v2.ResourceId, attr resource.SyncOpAttrs) ([]*v2.Resource, *resource.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("Listing collaborators")

	collaborators, err := o.client.GetCollaborators(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Set collaborators cache
	err = o.cache.setCollaboratorsCache(ctx, attr.Session, collaborators)
	if err != nil {
		return nil, nil, err
	}

	rv := make([]*v2.Resource, len(collaborators))

	for i, collaborator := range collaborators {
		us, err := collaboratorResource(&collaborator)
		if err != nil {
			return nil, nil, err
		}
		rv[i] = us
	}

	return rv, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *collaboratorBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ resource.SyncOpAttrs) ([]*v2.Entitlement, *resource.SyncOpResults, error) {
	return nil, nil, nil
}

func (o *collaboratorBuilder) Grants(ctx context.Context, resource *v2.Resource, attr resource.SyncOpAttrs) ([]*v2.Grant, *resource.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	rv := make([]*v2.Grant, 0)
	principalID := resource.Id

	collaboratorIdStr := resource.Id.Resource
	collaboratorId, err := strconv.Atoi(collaboratorIdStr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert collaborator id to int: %w", err)
	}

	collaborator, err := o.cache.getCollaborator(ctx, attr.Session, collaboratorIdStr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get collaborator from cache: %w", err)
	}

	// Build for roles
	for _, role := range collaborator.Roles {
		if role.EnvironmentType != o.env.String() {
			continue
		}

		var roleResource *v2.Resource

		switch {
		case workato.IsBaseRole(role.RoleName):
			baseRole, err := workato.GetBaseRole(resource.DisplayName)
			if err != nil {
				l.Error("failed to get base role %s", zap.String("role_name", resource.DisplayName), zap.Error(err))
				return nil, nil, fmt.Errorf("failed to get base role: %w", err)
			}
			roleResource = &v2.Resource{
				Id: &v2.ResourceId{
					ResourceType: roleResourceType.Id,
					Resource:     baseRole.RoleName,
				},
			}
		case !o.disableCustomRolesSync:
			role := getRoleByName(ctx, attr.Session, role.RoleName)
			if role == nil {
				return rv, nil, uhttp.WrapErrors(codes.NotFound, fmt.Sprintf("role %s (%d) not found", role.Name, role.Id))
			}
			roleResource = &v2.Resource{
				Id: &v2.ResourceId{
					ResourceType: roleResourceType.Id,
					Resource:     strconv.Itoa(role.Id),
				},
			}
		default:
			l.Debug("skipping role %s because it is not a base role and custom roles sync is disabled", zap.String("role_name", role.RoleName))
			continue
		}

		newGrant := grant.NewGrant(
			roleResource,
			collaboratorHasRoleEntitlement,
			principalID,
			grant.WithGrantMetadata(map[string]interface{}{
				"environment_type": role.EnvironmentType,
			}),
		)
		rv = append(rv, newGrant)
	}

	collaboratorRoles, err := o.client.GetCollaboratorPrivileges(ctx, collaboratorId)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, nil, fmt.Errorf("failed to get collaborator privileges: %w", err)
		}
		l.Debug("Collaborator privileges not found, skipping", zap.Int("collaborator_id", collaboratorId))
		return rv, nil, nil
	}

	for _, collaboratorRole := range collaboratorRoles {
		if collaboratorRole.EnvironmentType != o.env.String() {
			l.Debug("Collaborator role environment type does not match, skipping", zap.String("collaborator_role_environment_type", collaboratorRole.EnvironmentType), zap.String("collaborator_role_name", collaboratorRole.Name))
			continue
		}

		// Build for privileges
		for keyGroup, values := range collaboratorRole.Privileges {
			for _, value := range values {
				privilegeID := workato.PrivilegeId(keyGroup, value)

				privilegeResource := &v2.Resource{
					Id: &v2.ResourceId{
						ResourceType: privilegeResourceType.Id,
						Resource:     privilegeID,
					},
				}

				// Collaborator only have privileges if a role is assigned to them
				// To update collaborator privileges, the role must be updated
				// privilege grants for roles implemented in role resource
				grantToCollaborator := grant.NewGrant(
					privilegeResource,
					assignedEntitlement,
					principalID,
					grant.WithAnnotation(&v2.GrantImmutable{}),
				)

				rv = append(rv, grantToCollaborator)
			}
		}

		// Build for folders
		for _, folderId := range collaboratorRole.FolderIDs {
			folderResource := &v2.Resource{
				Id: &v2.ResourceId{
					ResourceType: folderResourceType.Id,
					Resource:     strconv.Itoa(folderId),
				},
			}

			// Collaborator only access to the folder if a role have access
			// To update collaborator folder access, the role must be updated
			newGrant := grant.NewGrant(
				folderResource,
				collaboratorAccessEntitlement,
				principalID,
				grant.WithAnnotation(&v2.GrantImmutable{}),
			)
			rv = append(rv, newGrant)
		}
	}

	return rv, nil, nil
}

func newCollaboratorBuilder(client *client.WorkatoClient, env workato.Environment, disableCustomRolesSync bool) *collaboratorBuilder {
	return &collaboratorBuilder{
		client:                 client,
		cache:                  newCollaboratorCache(client, env),
		env:                    env,
		disableCustomRolesSync: disableCustomRolesSync,
	}
}

func collaboratorResource(collaborator *client.Collaborator) (*v2.Resource, error) {
	var userStatus = v2.UserTrait_Status_STATUS_ENABLED

	profile := map[string]interface{}{
		"id":         collaborator.Id,
		"email":      collaborator.Email,
		"name":       collaborator.Name,
		"externalId": collaborator.ExternalId,
		"createdAt":  collaborator.CreatedAt.String(),
		"grantType":  collaborator.GrantType,
		"timeZone":   collaborator.TimeZone,
	}

	traits := []resource.UserTraitOption{
		resource.WithUserProfile(profile),
		resource.WithStatus(userStatus),
		resource.WithEmail(collaborator.Email, true),
		resource.WithUserLogin(collaborator.Email),
		resource.WithUserLogin(collaborator.Email),
		resource.WithCreatedAt(collaborator.CreatedAt),
		resource.WithAccountType(v2.UserTrait_ACCOUNT_TYPE_HUMAN),
	}

	ret, err := resource.NewUserResource(
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
