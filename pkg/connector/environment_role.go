package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	sdkGrant "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type environmentRoleBuilder struct {
	client *client.WorkatoClient
	env    workato.Environment
}

func (o *environmentRoleBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return environmentRoleResourceType
}

func (o *environmentRoleBuilder) List(ctx context.Context, _ *v2.ResourceId, attr rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	roles, nextToken, rl, err := o.client.GetEnvironmentRoles(ctx, attr.PageToken.Token)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-workato: failed to list environment roles: %w", err)
	}

	if err := setEnvironmentRolesByNameCache(ctx, attr.Session, roles); err != nil {
		return nil, nil, err
	}

	var envs []workato.Environment
	if o.env == workato.All {
		envs = workato.AllEnvironments()
	} else {
		envs = []workato.Environment{o.env}
	}

	rv := make([]*v2.Resource, 0, len(roles)*len(envs))
	for _, targetEnv := range envs {
		for _, role := range roles {
			r, err := environmentRoleResource(&role, o.env, targetEnv)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, r)
		}
	}

	annos := annotations.Annotations{}
	annos.WithRateLimiting(rl)
	return rv, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil
}

func (o *environmentRoleBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	opts := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("%s has Collaborator", resource.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s has %s", resource.DisplayName, collaboratorResourceType.DisplayName)),
	}
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(resource, collaboratorHasRoleEntitlement, opts...),
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

	envRole, _, err := o.client.GetEnvironmentRole(ctx, roleID)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-workato: failed to fetch environment role %s: %w", roleID, err)
	}

	roles := []client.SimpleRole{{
		RoleName:        envRole.Name,
		EnvironmentType: envType.String(),
		RoleType:        "environment",
	}}

	rl, err := o.client.UpdateCollaboratorRoles(ctx, userID, roles)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-workato: failed to update collaborator roles: %w", err)
	}

	newGrant := sdkGrant.NewGrant(ent.Resource, collaboratorHasRoleEntitlement, principal.Id,
		sdkGrant.WithGrantMetadata(map[string]any{"environment_type": envType.String()}),
	)
	annos := annotations.Annotations{}
	annos.WithRateLimiting(rl)
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
