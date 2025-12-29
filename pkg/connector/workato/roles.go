package workato

import (
	"fmt"
)

type Role struct {
	RoleName   string              `json:"role_name"`
	Privileges []CompoundPrivilege `json:"privileges"`
}

var BaseRoles = []Role{
	AdminRole,
	AnalystRole,
	OperatorRole,
}

func IsBaseRole(compare string) bool {
	for _, role := range BaseRoles {
		if role.RoleName == compare {
			return true
		}
	}
	return false
}

func GetBaseRole(compare string) (*Role, error) {
	switch compare {
	case "Admin":
		return &AdminRole, nil
	case "Analyst":
		return &AnalystRole, nil
	case "Operator":
		return &OperatorRole, nil
	default:
		return nil, fmt.Errorf("base role %s not found", compare)
	}
}

var AdminRole = Role{
	RoleName:   "Admin",
	Privileges: AllCompoundPrivileges(),
}

var AnalystRole = Role{
	RoleName:   "Analyst",
	Privileges: FindRelatedPrivileges(analystBaseRoles),
}

var OperatorRole = Role{
	RoleName:   "Operator",
	Privileges: FindRelatedPrivileges(operatorBaseRoles),
}

var analystBaseRoles = map[string][]string{
	"Runtime user connections": {
		"read",
		"update",
		"delete",
	},
	"Event streams": {
		"read",
		"create",
		"update",
		"delete",
		"view_history",
	},
	"Lookup tables": {
		"read",
		"create",
		"update_records",
		"delete",
		"update_schema",
	},
	"People task": {
		"all",
	},
	"Recipes": {
		"read",
		"create",
		"update",
		"delete",
		"run",
		"read_run_history",
	},
	"Folders": {
		"read",
		"create",
		"update",
		"delete",
	},
	"Projects": {
		"read",
		"create",
		"update",
		"delete",
	},
	"Connections": {
		"read",
		"create",
		"update",
		"delete",
	},
	"Connector SDK": {
		"all",
	},
	"Use in recipes": {
		"all",
	},
	"On-prem groups & agents": {
		"read",
		"create",
		"update",
		"delete",
	},
	"Connection - on-prem files": {
		"all",
	},
	"Connection - command line scripts": {
		"all",
	},
	"Project folder": {
		"all",
	},
	"Connection Folders": {
		"all",
	},
	"Common data models": {
		"read",
		"create",
		"update",
		"delete",
	},
	"Message templates": {
		"read",
		"create",
		"update",
		"delete",
	},
	"Workbot": {
		"read",
		"create",
		"update",
		"delete",
	},
	"Job History Search": {
		"read",
		"create",
		"update",
		"delete",
	},
	"Test automation": {
		"read",
		"manage_test_cases",
	},
	"Data masking": {
		"all",
	},
	"Environment properties": {
		"read",
		"update_records",
		"create",
		"delete",
	},
	"Project properties": {
		"read",
		"update_records",
		"create",
		"delete",
	},
	"Secrets management": {
		"read",
	},
}

var operatorBaseRoles = map[string][]string{
	"Recipes": {
		"read",
		"run",
		"read_run_history",
	},
	"Folders": {
		"read",
	},
	"Projects": {
		"read",
	},
	"Use in recipes": {
		"all",
	},
	"Test automation": {
		"read",
	},
}
