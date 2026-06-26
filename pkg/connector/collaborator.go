package connector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/conductorone/baton-sdk/pkg/annotations"
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

var (
	_ connectorbuilder.ResourceSyncerV2  = (*collaboratorBuilder)(nil)
	_ connectorbuilder.AccountManagerV2  = (*collaboratorBuilder)(nil)
	_ connectorbuilder.ResourceDeleterV2 = (*collaboratorBuilder)(nil)
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

// CreateAccountCapabilityDetails declares the credential options for account
// creation. Workato invitations are email-based and carry no password, so the
// connector advertises the no-password option. Required for the SDK to detect
// AccountManagerV2.
func (o *collaboratorBuilder) CreateAccountCapabilityDetails(_ context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
	}, nil, nil
}

// CreateAccount invites a new collaborator to Workato.
//
// Flow:
//  1. If a collaborator with the email already exists in the tenant, return
//     AlreadyExistsResult (idempotent; the account-provisioning CI re-runs create).
//  2. Otherwise POST /api/member_invitations to send an invitation email. If the
//     email already belongs to a Workato user (AlreadyExists), fall back to
//     POST /api/members to add the existing user to the team directly.
//  3. Re-resolve the collaborator by email. If they are now a member, return
//     SuccessResult. If the invitation is still pending email acceptance (no member
//     row yet), return ActionRequiredResult with no Resource — never fabricate a
//     resource keyed on the email, which could not reconcile with the real
//     ID-keyed resource a later sync emits.
func (o *collaboratorBuilder) CreateAccount(
	ctx context.Context, accountInfo *v2.AccountInfo, _ *v2.LocalCredentialOptions,
) (connectorbuilder.CreateAccountResponse, []*v2.PlaintextData, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)
	profileMap := accountInfo.GetProfile().AsMap()

	email := accountInfo.GetLogin()
	if email == "" {
		email, _ = profileMap["email"].(string)
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil, nil, fmt.Errorf("baton-workato: create account: email is required")
	}

	name, _ := profileMap["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil, nil, fmt.Errorf("baton-workato: create account: name is required")
	}

	// Step 1: short-circuit if the collaborator already exists in this tenant.
	if existing, err := o.client.GetCollaboratorByEmail(ctx, email); err == nil {
		res, resErr := collaboratorResource(existing)
		if resErr != nil {
			return nil, nil, nil, resErr
		}
		l.Debug("collaborator already exists, returning AlreadyExistsResult", zap.String("email", email))
		return &v2.CreateAccountResponse_AlreadyExistsResult{Resource: res}, nil, nil, nil
	} else if !client.IsNotFoundError(err) {
		return nil, nil, nil, fmt.Errorf("baton-workato: create account: failed to look up collaborator %s: %w", email, err)
	}

	// Step 2: build and send the invitation. Tag each role's role_type by resolving
	// the name against the environment-roles catalog; unresolved names default to
	// privilege_group on Workato's side.
	inviteReq := buildInviteRequest(name, email, profileMap)
	resolveEnvRoleTypes(ctx, o.client, inviteReq.EnvRoles)

	err := o.client.InviteCollaborator(ctx, inviteReq)
	switch {
	case err == nil:
		// invitation sent.
	case client.IsAlreadyExistsError(err):
		// Email already belongs to a Workato user outside this tenant: add them directly.
		l.Debug("invite reported already-exists, adding existing Workato user to team", zap.String("email", email))
		if addErr := o.client.AddExistingCollaborator(ctx, email); addErr != nil {
			if client.IsAlreadyExistsError(addErr) {
				// Already a member after all; fall through to resolution below.
				break
			}
			return nil, nil, nil, fmt.Errorf("baton-workato: create account: failed to add existing collaborator %s: %w", email, addErr)
		}
	default:
		return nil, nil, nil, fmt.Errorf("baton-workato: create account: failed to invite collaborator %s: %w", email, err)
	}

	// Step 3: resolve the resulting collaborator.
	created, err := o.client.GetCollaboratorByEmail(ctx, email)
	if err != nil {
		if client.IsNotFoundError(err) {
			// Invitation sent but not yet accepted — no stable member ID exists yet.
			return &v2.CreateAccountResponse_ActionRequiredResult{
				Message:               fmt.Sprintf("Invitation sent to %s. The collaborator must accept the email invitation to complete account creation.", email),
				IsCreateAccountResult: true,
			}, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("baton-workato: create account: failed to resolve collaborator %s: %w", email, err)
	}

	res, err := collaboratorResource(created)
	if err != nil {
		return nil, nil, nil, err
	}

	return &v2.CreateAccountResponse_SuccessResult{
		Resource:              res,
		IsCreateAccountResult: true,
	}, nil, nil, nil
}

// Delete removes a collaborator from Workato. Workato has no soft-disable, so this
// is a hard delete. A not-found result (HTTP 404) is treated as success because the
// platform retries deletes and the collaborator may already be gone.
func (o *collaboratorBuilder) Delete(ctx context.Context, resourceID *v2.ResourceId, _ *v2.ResourceId) (annotations.Annotations, error) {
	collaboratorID, err := strconv.Atoi(resourceID.Resource)
	if err != nil {
		return nil, fmt.Errorf("baton-workato: delete collaborator: invalid id %q: %w", resourceID.Resource, err)
	}

	err = o.client.DeleteCollaborator(ctx, collaboratorID)
	if err != nil {
		if client.IsNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("baton-workato: delete collaborator %d: %w", collaboratorID, err)
	}

	return nil, nil
}

// buildInviteRequest assembles the POST /api/member_invitations body from the
// account-creation profile. env_roles is built from per-environment role-name
// fields (dev_role/test_role/prod_role) and user_group_ids from a comma-separated
// string of group ids. All are optional but Workato generally needs at least one
// role to make the invite usable, matching the official Workato Team API and the
// connector's own UpdateCollaboratorRoles shape.
func buildInviteRequest(name, email string, profileMap map[string]interface{}) client.InviteCollaboratorRequest {
	req := client.InviteCollaboratorRequest{
		Name:  name,
		Email: email,
	}

	envRoleFields := []struct {
		key string
		env string
	}{
		{"dev_role", "dev"},
		{"test_role", "test"},
		{"prod_role", "prod"},
	}

	for _, f := range envRoleFields {
		roleName := optionalStringField(profileMap, f.key)
		if roleName == "" {
			continue
		}
		req.EnvRoles = append(req.EnvRoles, client.InviteEnvRole{
			EnvironmentType: f.env,
			Name:            roleName,
		})
	}

	req.UserGroupIDs = optionalStringListField(profileMap, "user_group_ids")

	return req
}

// environmentRoleResolver looks up an environment role by name. *client.WorkatoClient
// satisfies it; tests use a fake. Kept narrow so resolveEnvRoleTypes is unit-testable
// without a live API or the concrete client.
type environmentRoleResolver interface {
	GetEnvironmentRoleByName(ctx context.Context, name string) (*client.EnvironmentRole, annotations.Annotations, error)
}

// resolveEnvRoleTypes tags each invite role with its role_type. A role name that
// resolves to an environment role gets RoleType "environment"; everything else is
// left empty so Workato applies its "privilege_group" default. This brings the
// invite path to parity with the grant flow (role.go vs environment_role.go), which
// already distinguishes the two role types.
//
// Lookups are deduplicated by name and degrade gracefully: a lookup error is logged
// and the role falls back to the privilege_group default (the connector's prior
// behavior), so a transient environment_roles failure never breaks an otherwise
// valid privilege-group invite.
func resolveEnvRoleTypes(ctx context.Context, resolver environmentRoleResolver, roles []client.InviteEnvRole) {
	l := ctxzap.Extract(ctx)
	isEnvRole := make(map[string]bool, len(roles))

	for i := range roles {
		name := roles[i].Name

		resolved, seen := isEnvRole[name]
		if !seen {
			envRole, _, err := resolver.GetEnvironmentRoleByName(ctx, name)
			if err != nil {
				l.Debug("baton-workato: create account: environment role lookup failed; defaulting to privilege_group",
					zap.String("role_name", name), zap.Error(err))
				isEnvRole[name] = false
				continue
			}
			resolved = envRole != nil
			isEnvRole[name] = resolved
		}

		if resolved {
			roles[i].RoleType = roleTypeEnvironment
		}
	}
}

// optionalStringField returns a trimmed string profile field, or "" when absent.
func optionalStringField(profileMap map[string]interface{}, key string) string {
	raw, _ := profileMap[key].(string)
	return strings.TrimSpace(raw)
}

// optionalStringListField parses a comma-separated string profile field into a
// slice, trimming spaces and dropping empties. Returns a nil slice when the field
// is absent or empty.
func optionalStringListField(profileMap map[string]interface{}, key string) []string {
	raw, _ := profileMap[key].(string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var values []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values = append(values, part)
	}
	return values
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
