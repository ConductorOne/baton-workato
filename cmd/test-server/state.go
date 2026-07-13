package main

import (
	"slices"
	"sync"
	"time"
)

// The structs below mirror the JSON shapes the baton-workato client decodes
// (see pkg/connector/client/models.go). They are intentionally duplicated here
// so the test server has no dependency on the connector packages.

// SimpleRole is a per-environment role assignment on a collaborator. RoleType is
// "environment" for environment-type roles and empty (privilege_group) otherwise,
// mirroring what the connector sends and what list-collaborators returns.
// https://docs.workato.com/workato-api/team.html#list-collaborators
type SimpleRole struct {
	EnvironmentType string `json:"environment_type"`
	RoleName        string `json:"role_name"`
	RoleType        string `json:"role_type,omitempty"`
}

// Collaborator mirrors a Workato team member.
// https://docs.workato.com/workato-api/team.html#list-collaborators
type Collaborator struct {
	Id         int          `json:"id"`
	GrantType  string       `json:"grant_type"`
	Roles      []SimpleRole `json:"roles"`
	ExternalId string       `json:"external_id"`
	Name       string       `json:"name"`
	Email      string       `json:"email"`
	TimeZone   string       `json:"time_zone"`
	CreatedAt  time.Time    `json:"created_at"`
}

// Role mirrors a Workato custom role.
// https://docs.workato.com/workato-api/roles.html#list-roles
type Role struct {
	Id          int                 `json:"id"`
	Name        string              `json:"name"`
	Inheritable bool                `json:"inheritable"`
	FolderIDs   []int               `json:"folder_ids"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Privileges  map[string][]string `json:"privileges"`
}

// EnvironmentRole mirrors a Workato environment role. The connector resolves an
// invite role name against these to decide its role_type, and syncs them as the
// environment_role resource type.
// https://docs.workato.com/workato-api/environment-roles.html#list-environment-roles
type EnvironmentRole struct {
	Id           int       `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	MembersCount int       `json:"members_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Folder mirrors a Workato folder.
// https://docs.workato.com/workato-api/folders.html#list-folders
type Folder struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
	ParentId  int       `json:"parent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Project mirrors a Workato project.
// https://docs.workato.com/workato-api/projects.html#list-projects
type Project struct {
	Id          int    `json:"id"`
	Description string `json:"description"`
	FolderId    int    `json:"folder_id"`
	Name        string `json:"name"`
}

// State is the in-memory store. Every access goes through a method that holds
// the mutex so parallel CI requests can't race.
type State struct {
	mu sync.Mutex

	members     map[int]*Collaborator
	memberOrder []int
	nextID      int

	roles            []Role
	environmentRoles []EnvironmentRole
	folders          []Folder
	projects         []Project
}

var seedTime = time.Date(2024, time.January, 2, 15, 4, 5, 0, time.UTC)

// NewState returns a freshly seeded store. CI spins up a new server (and thus a
// new seed) per job, so there is deliberately no reset endpoint.
func NewState() *State {
	s := &State{
		members: make(map[int]*Collaborator),
		nextID:  1000,
	}
	seed(s)
	return s
}

func seed(s *State) {
	// Members: base roles only (Admin/Operator) so collaborator role-grant
	// emission resolves without depending on synced custom roles; carol has no
	// roles to exercise the empty-grants path.
	s.createMemberLocked("Alice Admin", "alice@example.com", []SimpleRole{
		{EnvironmentType: "dev", RoleName: "Admin"},
	})
	s.createMemberLocked("Bob Operator", "bob@example.com", []SimpleRole{
		{EnvironmentType: "prod", RoleName: "Operator"},
	})
	s.createMemberLocked("Carol NoRoles", "carol@example.com", nil)

	s.roles = []Role{
		{Id: 1, Name: "Custom Reviewer", Inheritable: false, FolderIDs: []int{}, CreatedAt: seedTime, UpdatedAt: seedTime, Privileges: map[string][]string{}},
		{Id: 2, Name: "Custom Auditor", Inheritable: true, FolderIDs: []int{}, CreatedAt: seedTime, UpdatedAt: seedTime, Privileges: map[string][]string{}},
	}

	// Environment roles let the invite path resolve a role name to role_type
	// "environment"; names absent here (e.g. "Admin") fall back to privilege_group.
	s.environmentRoles = []EnvironmentRole{
		{Id: 50, Name: "Deployer", Type: "environment", MembersCount: 0, CreatedAt: seedTime, UpdatedAt: seedTime},
		{Id: 51, Name: "Releaser", Type: "environment", MembersCount: 0, CreatedAt: seedTime, UpdatedAt: seedTime},
	}

	s.folders = []Folder{
		{Id: 10, Name: "Home", ParentId: 0, CreatedAt: seedTime, UpdatedAt: seedTime},
		{Id: 11, Name: "Shared", ParentId: 10, CreatedAt: seedTime, UpdatedAt: seedTime},
	}

	s.projects = []Project{
		{Id: 100, Name: "Billing Pipelines", Description: "Finance integrations", FolderId: 10},
		{Id: 101, Name: "HR Pipelines", Description: "People integrations", FolderId: 11},
	}
}

// createMemberLocked assumes the caller already holds s.mu.
func (s *State) createMemberLocked(name, email string, roles []SimpleRole) *Collaborator {
	id := s.nextID
	s.nextID++
	m := &Collaborator{
		Id:         id,
		GrantType:  "user",
		Roles:      roles,
		ExternalId: "",
		Name:       name,
		Email:      email,
		TimeZone:   "UTC",
		CreatedAt:  seedTime,
	}
	s.members[id] = m
	s.memberOrder = append(s.memberOrder, id)
	return m
}

// ListMembers returns copies of all members in insertion order.
func (s *State) ListMembers() []Collaborator {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Collaborator, 0, len(s.memberOrder))
	for _, id := range s.memberOrder {
		out = append(out, *s.members[id])
	}
	return out
}

// GetMemberByEmail returns the member with the given email (case-insensitive).
func (s *State) GetMemberByEmail(email string) (Collaborator, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range s.memberOrder {
		if equalFold(s.members[id].Email, email) {
			return *s.members[id], true
		}
	}
	return Collaborator{}, false
}

// CreateMember adds a new member and returns a copy. The bool reports whether a
// member with that email already existed (caller maps this to HTTP 409).
func (s *State) CreateMember(name, email string, roles []SimpleRole) (Collaborator, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range s.memberOrder {
		if equalFold(s.members[id].Email, email) {
			return *s.members[id], true
		}
	}
	if name == "" {
		name = email
	}
	m := s.createMemberLocked(name, email, roles)
	return *m, false
}

// DeleteMember removes a member by id. Returns false when it does not exist.
func (s *State) DeleteMember(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.members[id]; !ok {
		return false
	}
	delete(s.members, id)
	s.memberOrder = slices.DeleteFunc(s.memberOrder, func(x int) bool { return x == id })
	return true
}

// UpdateMemberRoles applies PUT /api/members/:id env_roles semantics: roles in the
// environments named in the request are replaced; roles in environments not included
// are unaffected. This mirrors the selective-update examples in
// https://docs.workato.com/workato-api/team.html#update-collaborator-roles and the
// connector's contract comment on UpdateCollaboratorRoles (pkg/connector/client/
// colaborator.go). A full-set replacement here would hide cross-environment
// regressions behind green tests. Returns false when the member does not exist.
func (s *State) UpdateMemberRoles(id int, roles []SimpleRole) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[id]
	if !ok {
		return false
	}
	touched := make(map[string]bool, len(roles))
	for _, r := range roles {
		touched[r.EnvironmentType] = true
	}
	merged := slices.DeleteFunc(slices.Clone(m.Roles), func(r SimpleRole) bool { return touched[r.EnvironmentType] })
	merged = append(merged, roles...)
	m.Roles = merged
	return true
}

// GetMember returns a copy of the member by id, mirroring GET /api/members/:id.
func (s *State) GetMember(id int) (Collaborator, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[id]
	if !ok {
		return Collaborator{}, false
	}
	return *m, true
}

// EnvironmentRoleByID returns a copy of the environment role by id, mirroring
// GET /api/environment_roles/:id — the lookup the connector's environment-role
// Grant path performs before PUT /api/members/:id.
func (s *State) EnvironmentRoleByID(id int) (EnvironmentRole, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.environmentRoles {
		if r.Id == id {
			return r, true
		}
	}
	return EnvironmentRole{}, false
}

// rolePrivilegeSeed maps a role name to the privilege matrix and folder scope that
// GET /api/members/:id/privileges reports for a member holding that role. The live
// response carries a folder_ids field per row that Workato's published docs omit
// (verified 2026-07-06 against a developer-sandbox tenant); the connector builds
// collaborator folder grants from it, so the mock must populate it to keep the
// privilege-grant and folder-grant paths covered.
var rolePrivilegeSeed = map[string]struct {
	privileges map[string][]string
	folderIDs  []int
}{
	"Admin":    {privileges: map[string][]string{"Recipes": {"read", "run"}, "Folders": {"read"}}, folderIDs: []int{10}},
	"Operator": {privileges: map[string][]string{"Recipes": {"read"}, "Folders": {"read"}}, folderIDs: []int{11}},
	"Deployer": {privileges: map[string][]string{"Recipes": {"read", "run"}}},
	"Releaser": {privileges: map[string][]string{"Recipes": {"read"}}},
}

// MemberPrivileges derives the /privileges rows from the member's current roles,
// one row per role, mirroring the live API's shape. Members with no roles produce
// an empty list (the connector maps that to NotFound and skips privilege grants).
// Returns false when the member does not exist.
func (s *State) MemberPrivileges(id int) ([]CollaboratorPrivilege, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[id]
	if !ok {
		return nil, false
	}
	rows := make([]CollaboratorPrivilege, 0, len(m.Roles))
	for _, r := range m.Roles {
		seedRow, ok := rolePrivilegeSeed[r.RoleName]
		if !ok {
			continue
		}
		rows = append(rows, CollaboratorPrivilege{
			EnvironmentType: r.EnvironmentType,
			Name:            r.RoleName,
			Privileges:      seedRow.privileges,
			FolderIDs:       seedRow.folderIDs,
		})
	}
	return rows, true
}

// Roles / Folders / Projects return copies of the seeded slices.
func (s *State) Roles() []Role {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.roles)
}

func (s *State) EnvironmentRoles() []EnvironmentRole {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.environmentRoles)
}

func (s *State) Folders() []Folder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.folders)
}

func (s *State) Projects() []Project {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.projects)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return toLower(a) == toLower(b)
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
