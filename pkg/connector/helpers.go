package connector

import (
	"fmt"
	"strings"

	"github.com/conductorone/baton-workato/pkg/connector/workato"
)

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
