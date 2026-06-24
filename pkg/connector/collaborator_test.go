package connector

import (
	"reflect"
	"testing"

	"github.com/conductorone/baton-workato/pkg/connector/client"
)

func TestOptionalStringListField(t *testing.T) {
	tests := []struct {
		name     string
		profile  map[string]interface{}
		key      string
		expected []string
	}{
		{"absent key", map[string]interface{}{}, "user_group_ids", nil},
		{"empty string", map[string]interface{}{"user_group_ids": ""}, "user_group_ids", nil},
		{"whitespace only", map[string]interface{}{"user_group_ids": "   "}, "user_group_ids", nil},
		{"non-string value", map[string]interface{}{"user_group_ids": 42}, "user_group_ids", nil},
		{"single value", map[string]interface{}{"user_group_ids": "g1"}, "user_group_ids", []string{"g1"}},
		{"comma-separated with spaces", map[string]interface{}{"user_group_ids": " g1 , g2 ,g3"}, "user_group_ids", []string{"g1", "g2", "g3"}},
		{"empty entries skipped", map[string]interface{}{"user_group_ids": "g1,,, g2 ,"}, "user_group_ids", []string{"g1", "g2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optionalStringListField(tt.profile, tt.key)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("optionalStringListField() = %#v, want %#v", got, tt.expected)
			}
		})
	}
}

func TestBuildInviteRequest(t *testing.T) {
	tests := []struct {
		name     string
		inName   string
		email    string
		profile  map[string]interface{}
		expected client.InviteCollaboratorRequest
	}{
		{
			name:     "minimal — no roles, no groups",
			inName:   "New Hire",
			email:    "new@example.com",
			profile:  map[string]interface{}{},
			expected: client.InviteCollaboratorRequest{Name: "New Hire", Email: "new@example.com"},
		},
		{
			name:    "all three env roles in dev/test/prod order",
			inName:  "Admin User",
			email:   "admin@example.com",
			profile: map[string]interface{}{"dev_role": "Admin", "test_role": "Operator", "prod_role": "Analyst"},
			expected: client.InviteCollaboratorRequest{
				Name:  "Admin User",
				Email: "admin@example.com",
				EnvRoles: []client.InviteEnvRole{
					{EnvironmentType: "dev", Name: "Admin"},
					{EnvironmentType: "test", Name: "Operator"},
					{EnvironmentType: "prod", Name: "Analyst"},
				},
			},
		},
		{
			name:    "subset — only prod_role; empty roles skipped",
			inName:  "Prod Only",
			email:   "prod@example.com",
			profile: map[string]interface{}{"dev_role": "", "prod_role": "Analyst"},
			expected: client.InviteCollaboratorRequest{
				Name:     "Prod Only",
				Email:    "prod@example.com",
				EnvRoles: []client.InviteEnvRole{{EnvironmentType: "prod", Name: "Analyst"}},
			},
		},
		{
			name:    "roles + user_group_ids together",
			inName:  "Grouped",
			email:   "grouped@example.com",
			profile: map[string]interface{}{"dev_role": "Admin", "user_group_ids": " 10 , 20 "},
			expected: client.InviteCollaboratorRequest{
				Name:         "Grouped",
				Email:        "grouped@example.com",
				UserGroupIDs: []string{"10", "20"},
				EnvRoles:     []client.InviteEnvRole{{EnvironmentType: "dev", Name: "Admin"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildInviteRequest(tt.inName, tt.email, tt.profile)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("buildInviteRequest() = %#v, want %#v", got, tt.expected)
			}
		})
	}
}
