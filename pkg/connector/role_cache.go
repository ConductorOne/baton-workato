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
	err := session.SetManyJSON(ctx, sessionStorage, parseJSONRolesCache(roles), sessions.WithPrefix("roles"))
	if err != nil {
		return fmt.Errorf("failed to set roles cache in session storage: %w", err)
	}

	for _, role := range roles {
		for _, folderID := range role.FolderIDs {
			folderIDStr := strconv.Itoa(folderID)
			folderRoles, found, err := session.GetJSON[[]*client.Role](ctx, sessionStorage, folderIDStr, sessions.WithPrefix("folder_roles"))
			if err != nil {
				return fmt.Errorf("failed to get folder roles from session storage: %w", err)
			}

			if !found {
				folderRoles = []*client.Role{}
			}

			copyRole := role
			folderRoles = append(folderRoles, &copyRole)

			err = session.SetJSON(ctx, sessionStorage, folderIDStr, folderRoles, sessions.WithPrefix("folder_roles"))
			if err != nil {
				return fmt.Errorf("failed to set folder roles in session storage: %w", err)
			}
		}
	}

	return nil
}

func parseJSONRolesCache(roles []client.Role) map[string]*client.Role {
	rolesMap := make(map[string]*client.Role)
	for _, role := range roles {
		roleIDStr := strconv.Itoa(role.Id)
		rolesMap[roleIDStr] = &role
	}
	return rolesMap
}
