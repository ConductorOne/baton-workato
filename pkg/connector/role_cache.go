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
	rolesCachePrefix       = "roles"
	folderRolesCachePrefix = "folder_roles"
)

func getRoleByFolder(ctx context.Context, sessionStorage sessions.SessionStore, folderID string) []*client.Role {
	l := ctxzap.Extract(ctx)

	folderRoles, found, err := session.GetJSON[[]*client.Role](ctx, sessionStorage, folderID, sessions.WithPrefix(folderRolesCachePrefix))
	if err != nil {
		l.Error("failed to get folder roles from session storage", zap.Error(err))
		return nil
	}

	if !found {
		return nil
	}

	return folderRoles
}

func getRoleById(ctx context.Context, sessionStorage sessions.SessionStore, roleID string) *client.Role {
	l := ctxzap.Extract(ctx)

	role, found, err := session.GetJSON[*client.Role](ctx, sessionStorage, roleID, sessions.WithPrefix(rolesCachePrefix))
	if err != nil {
		l.Error("failed to get role by id from session storage", zap.Error(err))
		return nil
	}

	if !found {
		return nil
	}

	return role
}

func setRolesCache(ctx context.Context, sessionStorage sessions.SessionStore, roles []client.Role) error {
	if len(roles) > 0 {
		err := session.SetManyJSON(ctx, sessionStorage, parseJSONRolesCache(roles), sessions.WithPrefix(rolesCachePrefix))
		if err != nil {
			return fmt.Errorf("failed to set roles cache in session storage: %w", err)
		}
	}

	var mapRoles = make(map[string][]*client.Role)

	for _, role := range roles {
		for _, folderID := range role.FolderIDs {
			folderIDStr := strconv.Itoa(folderID)
			copyRole := role

			if roles, ok := mapRoles[folderIDStr]; ok {
				roles = append(roles, &copyRole)
				mapRoles[folderIDStr] = roles
			} else {
				mapRoles[folderIDStr] = []*client.Role{&copyRole}
			}
		}
	}

	if (len(mapRoles)) > 0 {
		err := session.SetManyJSON(ctx, sessionStorage, mapRoles, sessions.WithPrefix(folderRolesCachePrefix))
		if err != nil {
			return fmt.Errorf("failed to set folder roles in session storage: %w", err)
		}
	}

	return nil
}

func parseJSONRolesCache(roles []client.Role) map[string]*client.Role {
	rolesMap := make(map[string]*client.Role)
	for _, role := range roles {
		roleCopy := role

		roleIDStr := strconv.Itoa(roleCopy.Id)
		rolesMap[roleIDStr] = &roleCopy
	}
	return rolesMap
}
