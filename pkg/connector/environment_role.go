package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	sdkGrant "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ connectorbuilder.ResourceSyncerV2 = (*environmentRoleBuilder)(nil)
var _ connectorbuilder.ResourceProvisionerV2Limited = (*environmentRoleBuilder)(nil)
var _ connectorbuilder.StaticEntitlementSyncerV2 = (*environmentRoleBuilder)(nil)

type environmentRoleBuilder struct {
	client *client.WorkatoClient
	env    workato.Environment
}

func (o *environmentRoleBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return environmentRoleResourceType
}

func (o *environmentRoleBuilder) List(ctx context.Context, _ *v2.ResourceId, attr rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	roles, nextToken, annos, err := o.client.GetEnvironmentRoles(ctx, attr.PageToken.Token)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: annos}, fmt.Errorf("baton-workato: failed to list environment roles: %w", err)
	}

	if err := setEnvironmentRolesByNameCache(ctx, attr.Session, roles); err != nil {
		return nil, &rs.SyncOpResults{Annotations: annos}, err
	}

	envs := resolveEnvironments(o.env)

	rv := make([]*v2.Resource, 0, len(roles)*len(envs))
	for _, targetEnv := range envs {
		for _, role := range roles {
			r, err := environmentRoleResource(role, o.env, targetEnv)
			if err != nil {
				return nil, &rs.SyncOpResults{Annotations: annos}, err
			}
			rv = append(rv, r)
		}
	}

	return rv, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil
}

func (o *environmentRoleBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

func (o *environmentRoleBuilder) StaticEntitlements(_ context.Context, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			nil,
			collaboratorHasRoleEntitlement,
			entitlement.WithGrantableTo(collaboratorResourceType),
			entitlement.WithDisplayName("Has environment role"),
			entitlement.WithDescription("Collaborator is assigned this environment role"),
		),
	}, nil, nil
}

func (o *environmentRoleBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

func (o *environmentRoleBuilder) Grant(ctx context.Context, principal *v2.Resource, ent *v2.Entitlement) ([]*v2.Grant, annotations.Annotations, error) {
	if principal.Id.ResourceType != collaboratorResourceType.Id {
		return nil, nil, fmt.Errorf("baton-workato: grant not supported for principal type %s", principal.Id.ResourceType)
	}

	userID, err := strconv.Atoi(principal.Id.Resource)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-workato: failed to parse collaborator id: %w", err)
	}

	roleID, envType, err := parseRoleResourceID(ent.Resource.Id.Resource, o.env)
	if err != nil {
		return nil, nil, err
	}

	envRole, annos, err := o.client.GetEnvironmentRole(ctx, roleID)
	if err != nil {
		return nil, annos, fmt.Errorf("baton-workato: failed to fetch environment role %s: %w", roleID, err)
	}

	roles := []client.SimpleRole{{
		RoleName:        envRole.Name,
		EnvironmentType: envType.String(),
		RoleType:        roleTypeEnvironment,
	}}

	annos, err = o.client.UpdateCollaboratorRoles(ctx, userID, roles)
	if err != nil {
		return nil, annos, fmt.Errorf("baton-workato: failed to update collaborator roles: %w", err)
	}

	newGrant := sdkGrant.NewGrant(ent.Resource, collaboratorHasRoleEntitlement, principal.Id,
		sdkGrant.WithGrantMetadata(map[string]any{"environment_type": envType.String()}),
	)
	return []*v2.Grant{newGrant}, annos, nil
}

func (o *environmentRoleBuilder) Revoke(_ context.Context, _ *v2.Grant) (annotations.Annotations, error) {
	return nil, status.Errorf(codes.Unimplemented, "baton-workato: revoke is not supported for environment roles")
}

func newEnvironmentRoleBuilder(c *client.WorkatoClient, env workato.Environment) *environmentRoleBuilder {
	return &environmentRoleBuilder{
		client: c,
		env:    env,
	}
}

func environmentRoleResource(role *client.EnvironmentRole, envConfig workato.Environment, targetEnv workato.Environment) (*v2.Resource, error) {
	if targetEnv == workato.All {
		return nil, fmt.Errorf("baton-workato: target environment %s is not supported for environment role resources", targetEnv.String())
	}

	id := strconv.Itoa(role.Id)
	profile := map[string]any{
		"id":          id,
		"name":        role.Name,
		"type":        role.Type,
		"environment": targetEnv.String(),
		"created_at":  role.CreatedAt.String(),
		"updated_at":  role.UpdatedAt.String(),
	}

	traits := []rs.RoleTraitOption{rs.WithRoleProfile(profile)}

	return rs.NewRoleResource(
		fmt.Sprintf("%s (%s)", role.Name, targetEnv.String()),
		environmentRoleResourceType,
		GetRoleResourceID(id, targetEnv, envConfig),
		traits,
	)
}
