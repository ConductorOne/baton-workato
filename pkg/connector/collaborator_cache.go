package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/session"
	"github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CompoundUser struct {
	User       *client.Collaborator
	UserDetail []*client.CollaboratorPrivilege
}

func (c *CompoundUser) Id() string {
	return strconv.Itoa(c.User.Id)
}

type collaboratorCache struct {
	client *client.WorkatoClient
	env    workato.Environment
}

func newCollaboratorCache(workatoClient *client.WorkatoClient, env workato.Environment) *collaboratorCache {
	return &collaboratorCache{
		client: workatoClient,
		env:    env,
	}
}

const (
	privilegeToUserCachePrefix = "privilege_to_user"
	folderToUserCachePrefix    = "folder_to_user"
	roleToUserCachePrefix      = "role_to_user"
)

func getUsersByPrivilege(ctx context.Context, sessionStorage sessions.SessionStore, privilegeKey string) []*CompoundUser {
	l := ctxzap.Extract(ctx)

	users, found, err := session.GetJSON[[]*CompoundUser](ctx, sessionStorage, privilegeKey, sessions.WithPrefix(privilegeToUserCachePrefix))
	if err != nil {
		l.Error("failed to get users by privilege from session storage", zap.Error(err))
		return nil
	}

	if !found {
		return nil
	}

	return users
}

func getUsersByFolder(ctx context.Context, sessionStorage sessions.SessionStore, folderId string) []*CompoundUser {
	l := ctxzap.Extract(ctx)

	users, found, err := session.GetJSON[[]*CompoundUser](ctx, sessionStorage, folderId, sessions.WithPrefix(folderToUserCachePrefix))
	if err != nil {
		l.Error("failed to get users by folder from session storage", zap.Error(err))
		return nil
	}

	if !found {
		return nil
	}

	return users
}

func getUsersByRole(ctx context.Context, sessionStorage sessions.SessionStore, roleName string) []*CompoundUser {
	l := ctxzap.Extract(ctx)

	users, found, err := session.GetJSON[[]*CompoundUser](ctx, sessionStorage, roleName, sessions.WithPrefix(roleToUserCachePrefix))
	if err != nil {
		l.Error("failed to get users by role from session storage", zap.Error(err))
		return nil
	}

	if !found {
		return nil
	}

	return users
}

func (c *collaboratorCache) setCollaboratorsCache(ctx context.Context, sessionStorage sessions.SessionStore, collaborators []client.Collaborator) error {
	l := ctxzap.Extract(ctx)

	for _, collaborator := range collaborators {
		collaboratorRoles, err := c.client.GetCollaboratorPrivileges(ctx, collaborator.Id)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				l.Warn("Collaborator not found, skipping", zap.Int("collaborator_id", collaborator.Id))
				continue
			}
			return err
		}

		compoundUser := &CompoundUser{
			User:       &collaborator,
			UserDetail: collaboratorRoles,
		}

		for _, collaboratorRole := range collaboratorRoles {
			if collaboratorRole.EnvironmentType != c.env.String() {
				continue
			}

			// Build for privileges
			for keyGroup, values := range collaboratorRole.Privileges {
				for _, value := range values {
					privilegeKey := workato.PrivilegeId(keyGroup, value)

					err = appendCachedValue[*CompoundUser](ctx, sessionStorage, privilegeToUserCachePrefix, privilegeKey, compoundUser)
					if err != nil {
						return fmt.Errorf("failed to set privilege to user cache in session storage: %w", err)
					}
				}
			}

			// Build for folders
			for _, folderId := range collaboratorRole.FolderIDs {
				folderIdStr := strconv.Itoa(folderId)
				err = appendCachedValue[*CompoundUser](ctx, sessionStorage, folderToUserCachePrefix, folderIdStr, compoundUser)
				if err != nil {
					return fmt.Errorf("failed to set folder to user cache in session storage: %w", err)
				}
			}
		}

		// Build for roles
		for _, role := range collaborator.Roles {
			if role.EnvironmentType != c.env.String() {
				continue
			}

			err = appendCachedValue[*CompoundUser](ctx, sessionStorage, roleToUserCachePrefix, role.RoleName, compoundUser)
			if err != nil {
				return fmt.Errorf("failed to set role to user cache in session storage: %w", err)
			}
		}
	}

	return nil
}

func appendCachedValue[T any](ctx context.Context, sessionStorage sessions.SessionStore, cachePrefix string, cacheKey string, cacheValue T) error {
	values, found, err := session.GetJSON[[]T](ctx, sessionStorage, cacheKey, sessions.WithPrefix(cachePrefix))
	if err != nil {
		return fmt.Errorf("failed to get cached value from session storage: %w", err)
	}

	if !found {
		values = []T{}
	}

	values = append(values, cacheValue)

	err = session.SetJSON(ctx, sessionStorage, cacheKey, values, sessions.WithPrefix(cachePrefix))
	if err != nil {
		return fmt.Errorf("failed to set folder roles in session storage: %w", err)
	}

	return nil
}
