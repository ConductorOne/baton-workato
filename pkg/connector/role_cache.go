package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/session"
	"github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const (
	rolesCachePrefix                  = "roles"
	folderRolesCachePrefix            = "folder_roles"
	rolesByNameCachePrefix            = "roles_by_name"
	environmentRolesByNameCachePrefix = "environment_roles_by_name"
)

func getRoleByFolder(ctx context.Context, sessionStorage sessions.SessionStore, folderID string) ([]*client.Role, error) {
	folderRoles, found, err := session.GetJSON[[]*client.Role](ctx, sessionStorage, folderID, sessions.WithPrefix(folderRolesCachePrefix))
	if err != nil {
		return nil, fmt.Errorf("baton-workato: failed to get folder roles from cache: %w", err)
	}
	if !found {
		return nil, nil
	}
	return folderRoles, nil
}

func getRoleById(ctx context.Context, sessionStorage sessions.SessionStore, roleID string) *client.Role {
	l := ctxzap.Extract(ctx)

	role, found, err := session.GetJSON[*client.Role](ctx, sessionStorage, roleID, sessions.WithPrefix(rolesCachePrefix))
	if err != nil {
		l.Error("failed to get role by id from session storage", zap.String("role_id", roleID), zap.Error(err))
		return nil
	}

	if !found {
		return nil
	}

	return role
}

func getRoleByName(ctx context.Context, sessionStorage sessions.SessionStore, roleName string) (*client.Role, error) {
	role, found, err := session.GetJSON[*client.Role](ctx, sessionStorage, roleName, sessions.WithPrefix(rolesByNameCachePrefix))
	if err != nil {
		return nil, fmt.Errorf("baton-workato: failed to get role by name from cache: %w", err)
	}
	if !found {
		return nil, nil
	}
	return role, nil
}

func setRolesCache(ctx context.Context, sessionStorage sessions.SessionStore, roles []*client.Role) error {
	if len(roles) > 0 {
		err := session.SetManyJSON(ctx, sessionStorage, parseJSONRolesCache(roles), sessions.WithPrefix(rolesCachePrefix))
		if err != nil {
			return fmt.Errorf("failed to set roles cache in session storage: %w", err)
		}
	}

	var mapRoles = make(map[string][]*client.Role)
	var mapRolesByName = make(map[string]*client.Role)

	for _, role := range roles {
		for _, folderID := range role.FolderIDs {
			folderIDStr := strconv.Itoa(folderID)
			if _, ok := mapRoles[folderIDStr]; !ok {
				existing, err := getRoleByFolder(ctx, sessionStorage, folderIDStr)
				if err != nil {
					return err
				}
				mapRoles[folderIDStr] = existing
			}
			mapRoles[folderIDStr] = append(mapRoles[folderIDStr], role)
		}

		mapRolesByName[role.Name] = role
	}

	if (len(mapRoles)) > 0 {
		err := session.SetManyJSON(ctx, sessionStorage, mapRoles, sessions.WithPrefix(folderRolesCachePrefix))
		if err != nil {
			return fmt.Errorf("failed to set folder roles in session storage: %w", err)
		}
	}

	if (len(mapRolesByName)) > 0 {
		err := session.SetManyJSON(ctx, sessionStorage, mapRolesByName, sessions.WithPrefix(rolesByNameCachePrefix))
		if err != nil {
			return fmt.Errorf("failed to set roles by name in session storage: %w", err)
		}
	}

	return nil
}

func setEnvironmentRolesByNameCache(ctx context.Context, sessionStorage sessions.SessionStore, roles []*client.EnvironmentRole) error {
	byName := make(map[string]*client.EnvironmentRole, len(roles))
	for _, role := range roles {
		byName[role.Name] = role
	}
	if len(byName) == 0 {
		return nil
	}
	if err := session.SetManyJSON(ctx, sessionStorage, byName, sessions.WithPrefix(environmentRolesByNameCachePrefix)); err != nil {
		return fmt.Errorf("baton-workato: failed to set environment roles cache: %w", err)
	}
	return nil
}

func getEnvironmentRoleByName(ctx context.Context, sessionStorage sessions.SessionStore, roleName string) (*client.EnvironmentRole, error) {
	role, found, err := session.GetJSON[*client.EnvironmentRole](ctx, sessionStorage, roleName, sessions.WithPrefix(environmentRolesByNameCachePrefix))
	if err != nil {
		return nil, fmt.Errorf("baton-workato: failed to get environment role by name from cache: %w", err)
	}
	if !found {
		return nil, nil
	}
	return role, nil
}

func parseJSONRolesCache(roles []*client.Role) map[string]*client.Role {
	rolesMap := make(map[string]*client.Role)
	for _, role := range roles {
		rolesMap[strconv.Itoa(role.Id)] = role
	}
	return rolesMap
}
