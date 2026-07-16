package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/annotations"
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

	// rolesCachePopulatedKey is a sentinel stored alongside the roles cache to
	// record that role.List (or a self-heal) has populated it for the current
	// sync session. Role ids are numeric, so this key never collides with a
	// cached role entry.
	rolesCachePopulatedKey = "__roles_cache_populated__"
)

// rolesCachePopulated reports whether the roles caches have been populated for
// the current sync session. On a fresh sync role.List populates them; on a
// resumed/restarted sync (which runs under a new sync id and skips re-listing)
// the sentinel is absent, which is the signal to self-heal.
func rolesCachePopulated(ctx context.Context, sessionStorage sessions.SessionStore) bool {
	populated, found, err := session.GetJSON[bool](ctx, sessionStorage, rolesCachePopulatedKey, sessions.WithPrefix(rolesCachePrefix))
	if err != nil {
		return false
	}
	return found && populated
}

// ensureRolesCache lazily populates the roles caches if they are absent for the
// current sync session. It is a no-op once populated, so it fetches at most
// once per session. This heals the case where a sync is resumed/restarted under
// a new sync id without re-running role.List, which otherwise leaves the Grants
// phase reading an empty roles cache (see CXP-629 grant-collapse investigation).
func ensureRolesCache(ctx context.Context, sessionStorage sessions.SessionStore, workatoClient *client.WorkatoClient) (annotations.Annotations, error) {
	if rolesCachePopulated(ctx, sessionStorage) {
		return nil, nil
	}

	l := ctxzap.Extract(ctx)
	l.Info("baton-workato: roles cache not populated for this sync session, self-healing by re-listing roles")

	roles, annos, err := fetchAllRoles(ctx, workatoClient)
	if err != nil {
		return annos, fmt.Errorf("baton-workato: failed to self-heal roles cache: %w", err)
	}

	// The self-heal holds the complete role set, so folder_roles is rebuilt from
	// scratch rather than merged into whatever is already cached. Merging on this
	// path double-counts roles when an earlier setRolesCache wrote folder_roles
	// but died before the populated sentinel (e.g. a transient session-store
	// write failure), and the retried self-heal appends the same roles again.
	return annos, setRolesCache(ctx, sessionStorage, roles, false)
}

// fetchAllRoles pages through GetRoles and returns every custom role along with
// the annotations from the final page (rate-limit signal). Workato exposes no
// get-role-by-id endpoint, so resolving a single role requires the list; the
// role set is small, so this is cheap.
func fetchAllRoles(ctx context.Context, workatoClient *client.WorkatoClient) ([]*client.Role, annotations.Annotations, error) {
	var all []*client.Role
	var annos annotations.Annotations
	token := ""
	for {
		roles, nextToken, pageAnnos, err := workatoClient.GetRoles(ctx, token)
		if err != nil {
			return nil, pageAnnos, wrapRolesAuthError(err)
		}
		annos = pageAnnos
		all = append(all, roles...)
		if nextToken == "" {
			break
		}
		token = nextToken
	}
	return all, annos, nil
}

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

// setRolesCache writes the roles, folder_roles, and roles_by_name caches and the
// populated sentinel. mergeFolderRoles controls how folder_roles is built: the
// paged role.List path passes true so each page appends onto folder_roles
// accumulated by prior pages; the self-heal path passes false because it holds
// the complete role set and rebuilding from scratch keeps a retry idempotent.
func setRolesCache(ctx context.Context, sessionStorage sessions.SessionStore, roles []*client.Role, mergeFolderRoles bool) error {
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
				var existing []*client.Role
				if mergeFolderRoles {
					var err error
					existing, err = getRoleByFolder(ctx, sessionStorage, folderIDStr)
					if err != nil {
						return err
					}
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

	// Mark the roles cache as populated for this sync session so the Grants phase
	// can tell an unpopulated cache (needs self-heal) from a folder/role that
	// legitimately has no match. Set even when there are no custom roles.
	if err := session.SetJSON(ctx, sessionStorage, rolesCachePopulatedKey, true, sessions.WithPrefix(rolesCachePrefix)); err != nil {
		return fmt.Errorf("failed to mark roles cache as populated: %w", err)
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
