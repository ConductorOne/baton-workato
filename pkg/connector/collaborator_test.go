package connector

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-workato/pkg/connector/client"
)

// TestResolveInviteEmail is the repro test for CXH-1958.
// Before the fix, GetLogin() was used first and profileMap["email"] was only a
// fallback for an empty login — so a handle login (non-email) would be sent
// verbatim to Workato's invite API, which rejects it with "400 Email is invalid".
func TestResolveInviteEmail(t *testing.T) {
	tests := []struct {
		name           string
		login          string
		emailAddresses []string
		profileMap     map[string]interface{}
		want           string
	}{
		{
			// Repro: C1 login is a handle (not an email). Mapped profile email must win.
			name:       "handle login + mapped email → mapped email wins",
			login:      "carolinaroncaglia",
			profileMap: map[string]interface{}{"email": "carolina@example.com"},
			want:       "carolina@example.com",
		},
		{
			// Mapped email preferred even when login is email-shaped.
			name:       "mapped email preferred over email-shaped login",
			login:      "user@example.com",
			profileMap: map[string]interface{}{"email": "mapped@example.com"},
			want:       "mapped@example.com",
		},
		{
			// Login is email-shaped and no profile email present → use login.
			name:       "email-shaped login, no mapped email → login used",
			login:      "user@example.com",
			profileMap: map[string]interface{}{},
			want:       "user@example.com",
		},
		{
			// Handle login, no mapped email, but GetEmails populated → use first address.
			name:           "handle login + GetEmails fallback",
			login:          "handle",
			profileMap:     map[string]interface{}{},
			emailAddresses: []string{"handle@company.com"},
			want:           "handle@company.com",
		},
		{
			// Nothing available → empty string; caller returns an error.
			name:       "all sources empty → empty string",
			login:      "",
			profileMap: map[string]interface{}{},
			want:       "",
		},
		{
			// Whitespace-only login is not email-shaped.
			name:       "whitespace-only login + no mapped email → empty",
			login:      "   ",
			profileMap: map[string]interface{}{},
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveInviteEmail(tt.login, tt.emailAddresses, tt.profileMap)
			if got != tt.want {
				t.Errorf("resolveInviteEmail() = %q, want %q", got, tt.want)
			}
		})
	}
}

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

// fakeEnvRoleResolver returns an environment role for any name in envRoleNames, and
// an error for any name in errNames (checked first), modelling a not-found as
// (nil, nil, nil) the way the real client does.
type fakeEnvRoleResolver struct {
	envRoleNames map[string]bool
	errNames     map[string]bool
	calls        map[string]int
}

func (f *fakeEnvRoleResolver) GetEnvironmentRoleByName(_ context.Context, name string) (*client.EnvironmentRole, annotations.Annotations, error) {
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[name]++
	if f.errNames[name] {
		return nil, nil, errors.New("lookup failed")
	}
	if f.envRoleNames[name] {
		return &client.EnvironmentRole{Name: name}, nil, nil
	}
	return nil, nil, nil
}

func TestResolveEnvRoleTypes(t *testing.T) {
	tests := []struct {
		name      string
		resolver  *fakeEnvRoleResolver
		roles     []client.InviteEnvRole
		wantTypes []string // expected RoleType per role, in order
	}{
		{
			name:      "privilege-group role (not found) keeps empty role_type",
			resolver:  &fakeEnvRoleResolver{},
			roles:     []client.InviteEnvRole{{EnvironmentType: "dev", Name: "Admin"}},
			wantTypes: []string{""},
		},
		{
			name:      "environment role gets role_type=environment",
			resolver:  &fakeEnvRoleResolver{envRoleNames: map[string]bool{"Deployer": true}},
			roles:     []client.InviteEnvRole{{EnvironmentType: "prod", Name: "Deployer"}},
			wantTypes: []string{roleTypeEnvironment},
		},
		{
			name:     "mixed: env role tagged, privilege group left default",
			resolver: &fakeEnvRoleResolver{envRoleNames: map[string]bool{"Deployer": true}},
			roles: []client.InviteEnvRole{
				{EnvironmentType: "dev", Name: "Admin"},
				{EnvironmentType: "prod", Name: "Deployer"},
			},
			wantTypes: []string{"", roleTypeEnvironment},
		},
		{
			name:      "lookup error degrades to privilege_group default",
			resolver:  &fakeEnvRoleResolver{errNames: map[string]bool{"Flaky": true}},
			roles:     []client.InviteEnvRole{{EnvironmentType: "dev", Name: "Flaky"}},
			wantTypes: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolveEnvRoleTypes(context.Background(), tt.resolver, tt.roles)
			for i, want := range tt.wantTypes {
				if tt.roles[i].RoleType != want {
					t.Errorf("role[%d] (%s) RoleType = %q, want %q", i, tt.roles[i].Name, tt.roles[i].RoleType, want)
				}
			}
		})
	}
}

func TestResolveEnvRoleTypesDeduplicatesLookups(t *testing.T) {
	resolver := &fakeEnvRoleResolver{envRoleNames: map[string]bool{"Deployer": true}}
	roles := []client.InviteEnvRole{
		{EnvironmentType: "dev", Name: "Deployer"},
		{EnvironmentType: "test", Name: "Deployer"},
		{EnvironmentType: "prod", Name: "Deployer"},
	}

	resolveEnvRoleTypes(context.Background(), resolver, roles)

	for i := range roles {
		if roles[i].RoleType != roleTypeEnvironment {
			t.Errorf("role[%d] RoleType = %q, want %q", i, roles[i].RoleType, roleTypeEnvironment)
		}
	}
	if got := resolver.calls["Deployer"]; got != 1 {
		t.Errorf("GetEnvironmentRoleByName called %d times for %q, want 1 (deduplicated)", got, "Deployer")
	}
}
