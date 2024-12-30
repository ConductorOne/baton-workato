package workato

import (
	"fmt"
	"slices"
)

type Privilege struct {
	Id          string
	Description string
}

type CompoundPrivilege struct {
	Resource  string
	Privilege Privilege
}

func PrivilegeId(group, privilege string) string {
	return fmt.Sprintf("%s-%s", group, privilege)
}

func (receiver CompoundPrivilege) Id() string {
	return PrivilegeId(receiver.Resource, receiver.Privilege.Id)
}

var (
	// Privileges https://docs.workato.com/privileges.html#privileges
	Privileges = map[string][]Privilege{
		"Runtime user connections": {
			Privilege{
				Id:          "read",
				Description: "View the Runtime user connections setting.",
			},
			Privilege{
				Id:          "edit",
				Description: "Edit the Runtime user connections setting.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete the Runtime user connections setting.",
			},
		},
		"Event streams": {
			Privilege{
				Id:          "read",
				Description: "View Event topics in the workspace.",
			},
			Privilege{
				Id:          "create",
				Description: "Create Event topics in the workspace.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit Event topics in the workspace.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete Event topics in the workspace.",
			},
			Privilege{
				Id:          "view_history",
				Description: "View the message content in the Event topics messages list.",
			},
		},
		"Lookup tables": {
			Privilege{
				Id:          "read",
				Description: "Allows users to view all tables and their records.",
			},
			Privilege{
				Id:          "create",
				Description: "Allows users to create new tables in the Lookup tables interface.",
			},
			Privilege{
				Id:          "update_records",
				Description: "Allows users to add, edit, or delete records for all Lookup tables in the Lookup tables interface.",
			},
			Privilege{
				Id:          "delete",
				Description: "Allows users to delete tables.",
			},
			Privilege{
				Id:          "update_schema",
				Description: "Allows users to edit the schema (to add, remove, or edit columns) for any table.",
			},
		},
		"People task": {
			Privilege{
				Id:          "All",
				Description: "Access to the People task tool.",
			},
		},
		"Recipes": {
			Privilege{
				Id:          "read",
				Description: "View recipes in a workspace.",
			},
			Privilege{
				Id:          "create",
				Description: "Create recipes in a workspace.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit recipes in a workspace.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete recipes in a workspace.",
			},
			Privilege{
				Id:          "run",
				Description: "Run recipes and start and stop recipe tests in a workspace.",
			},
			Privilege{
				Id:          "read_run_history",
				Description: "View a recipe's job history in the Jobs tab.",
			},
		},
		"Folders": {
			Privilege{
				Id:          "read",
				Description: "View folders and sub-folders in a workspace.",
			},
			Privilege{
				Id:          "create",
				Description: "Create folders and sub-folders in a workspace.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit folders and sub-folders in a workspace.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete folders and sub-folders in a workspace.",
			},
		},
		"Projects": {
			Privilege{
				Id:          "read",
				Description: "View specific projects in a workspace.",
			},
			Privilege{
				Id:          "create",
				Description: "Create projects in a workspace.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit projects in a workspace.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete projects in a workspace.",
			},
		},
		"Connections": {
			Privilege{
				Id:          "read",
				Description: "View connections in a workspace.",
			},
			Privilege{
				Id:          "create",
				Description: "Create connections in a workspace.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit connections in a workspace.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete connections in a workspace.",
			},
		},
		"Connector SDK": {
			Privilege{
				Id:          "all",
				Description: "Full Connector SDK permissions: view, edit, create, and delete.",
			},
		},
		"Use in recipes": {
			Privilege{
				Id:          "all",
				Description: "Allow users to distribute custom connectors into this workspace.",
			},
		},
		"On-prem groups & agents": {
			Privilege{
				Id:          "read",
				Description: "View on-prem groups and agents.",
			},
			Privilege{
				Id:          "create",
				Description: "Create on-prem groups and agents.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit on-prem groups and agents.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete on-prem groups and agents.",
			},
		},
		"Connection - on-prem files": {
			Privilege{
				Id:          "all",
				Description: "Access to create, edit, and delete on-prem files and on-prem files secondary connections.",
			},
		},
		"Connection - command line scripts": {
			Privilege{
				Id:          "all",
				Description: "Access to create, edit, and delete on-prem command line scripts connections.",
			},
		},
		"Project folder": {
			Privilege{
				Id:          "all",
				Description: "Access to create, edit, and delete project folders.",
			},
		},
		"Connection Folders": {
			Privilege{
				Id:          "all",
				Description: "Access to create, edit, and delete connection folders.",
			},
		},
		"Common data models": {
			Privilege{
				Id:          "read",
				Description: "View Common data models in the workspace.",
			},
			Privilege{
				Id:          "create",
				Description: "Create Common data models in the workspace.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit Common data models in the workspace.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete Common data models in the workspace.",
			},
		},
		"Message templates": {
			Privilege{
				Id:          "read",
				Description: "View Message templates in the workspace.",
			},
			Privilege{
				Id:          "create",
				Description: "Create Message templates in the workspace.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit Message templates in the workspace.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete Message templates in the workspace.",
			},
		},
		"Workbot": {
			Privilege{
				Id:          "read",
				Description: "View installed Workbots in the workspace.",
			},
			Privilege{
				Id:          "create",
				Description: "Create Workbots in the workspace.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit installed Workbots in the workspace.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete installed Workbots in the workspace.",
			},
		},
		"Job History Search": {
			Privilege{
				Id:          "read",
				Description: "View job history search results.",
			},
			Privilege{
				Id:          "create",
				Description: "Create job history search queries.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit job history search queries.",
			},
			Privilege{
				Id:          "delete",
				Description: "Delete job history search queries.",
			},
		},
		"Test automation": {
			Privilege{
				Id:          "read",
				Description: "View test case details, including mock data and checks.",
			},
			Privilege{
				Id: "manage_test_cases",
				Description: `View test case details

Create new test cases

Edit test cases

Pick data for mocks from previous jobs

Delete test cases

Run test cases`,
			},
		},
		"Data masking": {
			Privilege{
				Id:          "all",
				Description: `Access to create, edit, and delete data masking rules.`,
			},
		},
		"Environment properties": {
			Privilege{
				Id:          "read",
				Description: "Allows users to view all Environment properties.",
			},
			Privilege{
				Id:          "update_records",
				Description: "Allows users to add, edit, or delete Environment properties.",
			},
			Privilege{
				Id:          "create",
				Description: "Allows users to create new Environment properties.",
			},
			Privilege{
				Id:          "delete",
				Description: "Allows users to delete Environment properties.",
			},
		},
		"Project properties": {
			Privilege{
				Id:          "read",
				Description: "Allows users to view all project properties.",
			},
			Privilege{
				Id:          "update_records",
				Description: "Allows users to add, edit, or delete project properties.",
			},
			Privilege{
				Id:          "create",
				Description: "Allows users to create new project properties.",
			},
			Privilege{
				Id:          "delete",
				Description: "Allows users to delete project properties.",
			},
		},
		"Secrets management": {
			Privilege{
				Id:          "read",
				Description: "View secrets management details, including all secrets configured in your workspace.",
			},
			Privilege{
				Id:          "update",
				Description: "Edit secrets for your workspace.",
			},
		},
		"Activity audit": {
			Privilege{
				Id:          "all",
				Description: `Access to view workspace activity in the Dashboard's Activity audit log. This permission grants the user the ability to view all activity logs, regardless of other access settings.`,
			},
		},
		"Collaborator SAML SSO auth": {
			Privilege{
				Id:          "all",
				Description: `View and edit SAML SSO settings for the workspace.`,
			},
		},
		"Collaborators": {
			Privilege{
				Id:          "all",
				Description: "Manage collaborators in the workspace, including adding, editing, and removing collaborators.",
			},
		},
		"Recipe lifecycle management": {
			Privilege{
				Id:          "all",
				Description: `Access to the Recipe lifecycle management (RLCM) feature. This includes the ability to create manifests and view and interact with all assets included in manifests.`,
			},
		},
		"Collaborator roles (non-system)": {
			Privilege{
				Id:          "all",
				Description: "View, edit, create, and delete custom collaborator roles in the workspace.",
			},
		},
		"Developer API": {
			Privilege{
				Id:          "all",
				Description: "View and edit developer API settings for the workspace.",
			},
		},
		"Workspace settings": {
			Privilege{
				Id:          "all",
				Description: "View and edit various workspace settings.",
			},
		},
		"Debug, Log and Security": {
			Privilege{
				Id:          "all",
				Description: "Access to view and edit the workspace’s environment-specific settings, including error alerts, network trace, data retention, and AWS IAM information.",
			},
		},
		"Network trace": {
			Privilege{
				Id:          "all",
				Description: `View network traces in job histories. Includes recipe input, output, and the network trace of HTTP calls. HTTP call information includes HTTP headers, requests, and communication (responses) between Workato and the end application`,
			},
		},
	}
)

func AllCompoundPrivileges() []CompoundPrivilege {
	var all []CompoundPrivilege
	for resource, privileges := range Privileges {
		for _, privilege := range privileges {
			compoundPrivilege := CompoundPrivilege{
				Resource:  resource,
				Privilege: privilege,
			}

			all = append(all, compoundPrivilege)
		}
	}
	return all
}

func FindRelatedPrivileges(param map[string][]string) []CompoundPrivilege {
	all := make([]CompoundPrivilege, 0)

	for key, values := range param {
		if reference, ok := Privileges[key]; ok {
			for _, value := range values {

				// Since it's a small list, we can use a linear search¬¬¬¬
				index := slices.IndexFunc(reference, func(c Privilege) bool {
					return c.Id == value
				})

				if index >= 0 {
					temp := CompoundPrivilege{
						Resource:  key,
						Privilege: reference[index],
					}

					all = append(all, temp)
				}
			}
		}
	}

	return all
}
