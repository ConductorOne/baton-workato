package connector

import (
	"fmt"
	"strings"

	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	roleTypeEnvironment    = "environment"
	roleTypePrivilegeGroup = "privilege_group"
)

// isEnvironmentRolesAccessDenied reports whether err indicates the API key
// cannot access Workato's environment roles endpoints (GET /api/environment_roles).
//
// Environment roles are only available on workspaces using the new RBAC v2
// (environment-based) model and require a specific API key privilege. A key
// that is otherwise valid — collaborators, folders, projects, and custom roles
// all sync fine — can still receive 401/403 on the environment roles endpoint
// when it lacks that privilege. The connector's Validate() exercises the
// members endpoint, so by the time grants are syncing the credentials are
// known to be valid in general; a 401/403 scoped to environment roles is a
// missing-permission/feature gap, not a bad credential. In that case we degrade
// gracefully (skip environment role data) instead of failing the entire sync,
// mirroring how this package already skips custom roles and privileges that
// cannot be resolved.
func isEnvironmentRolesAccessDenied(err error) bool {
	switch status.Code(err) {
	case codes.Unauthenticated, codes.PermissionDenied:
		return true
	default:
		return false
	}
}

func resolveEnvironments(env workato.Environment) []workato.Environment {
	if env == workato.All {
		return workato.AllEnvironments()
	}
	return []workato.Environment{env}
}

func GetRoleResourceID(roleId string, targetEnv workato.Environment, configEnv workato.Environment) string {
	if configEnv != workato.All {
		return roleId
	}
	return fmt.Sprintf("%s-%s", roleId, targetEnv.String())
}

func parseRoleResourceID(resourceID string, configEnv workato.Environment) (string, workato.Environment, error) {
	if configEnv != workato.All {
		return resourceID, configEnv, nil
	}
	for _, e := range workato.AllEnvironments() {
		if id, found := strings.CutSuffix(resourceID, "-"+e.String()); found {
			return id, e, nil
		}
	}
	return "", workato.All, fmt.Errorf("baton-workato: cannot parse environment from resource ID %s", resourceID)
}
