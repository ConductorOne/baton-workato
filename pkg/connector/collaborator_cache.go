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

	var (
		roleToUsers      = make(map[string][]*CompoundUser)
		folderToUsers    = make(map[string][]*CompoundUser)
		privilegeToUsers = make(map[string][]*CompoundUser)
	)

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

					appendCachedValue(privilegeToUsers, privilegeKey, compoundUser)
				}
			}

			// Build for folders
			for _, folderId := range collaboratorRole.FolderIDs {
				folderIdStr := strconv.Itoa(folderId)

				appendCachedValue(folderToUsers, folderIdStr, compoundUser)
			}
		}

		// Build for roles
		for _, role := range collaborator.Roles {
			if role.EnvironmentType != c.env.String() {
				continue
			}
			appendCachedValue(roleToUsers, role.RoleName, compoundUser)
		}
	}

	if (len(privilegeToUsers)) > 0 {
		err := session.SetManyJSON(ctx, sessionStorage, privilegeToUsers, sessions.WithPrefix(privilegeToUserCachePrefix))
		if err != nil {
			return fmt.Errorf("failed to set privilege to user cache in session storage: %w", err)
		}
	}
	if (len(folderToUsers)) > 0 {
		err := session.SetManyJSON(ctx, sessionStorage, folderToUsers, sessions.WithPrefix(folderToUserCachePrefix))
		if err != nil {
			return fmt.Errorf("failed to set folder to user cache in session storage: %w", err)
		}
	}
	if (len(roleToUsers)) > 0 {
		err := session.SetManyJSON(ctx, sessionStorage, roleToUsers, sessions.WithPrefix(roleToUserCachePrefix))
		if err != nil {
			return fmt.Errorf("failed to set role to user cache in session storage: %w", err)
		}
	}

	return nil
}

func appendCachedValue[T any](storage map[string][]*T, cacheKey string, cacheValue *T) {
	storage[cacheKey] = append(storage[cacheKey], cacheValue)
}
