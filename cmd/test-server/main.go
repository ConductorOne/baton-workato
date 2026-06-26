// Command test-server is an in-memory mock of the Workato Team API. It serves
// the exact endpoints, JSON shapes, auth scheme and error envelope the
// baton-workato connector calls, so CI can exercise sync and account
// provisioning (invite -> sync -> delete) without a real Workato tenant.
//
// Run locally:
//
//	go run ./cmd/test-server/ &
//	baton-workato --workato-api-key test-token --workato-base-url http://localhost:8765 ...
//
// Every handler mirrors the documented Workato API, NOT the connector. If the
// connector and the docs disagree, the handler follows the docs so the mismatch
// surfaces in CI instead of being hidden.
// https://docs.workato.com/workato-api.html
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	listenAddr = ":8765"
	// testToken is the only credential the mock accepts. Real credentials never
	// belong in a test server.
	testToken = "test-token"
)

// commonPagination is the Workato `{ "data": [...], "total": N }` envelope used
// by the members and privileges endpoints.
type commonPagination[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
}

type inviteEnvRole struct {
	EnvironmentType string `json:"environment_type"`
	Name            string `json:"name"`
	RoleType        string `json:"role_type"`
}

type inviteRequest struct {
	Name         string          `json:"name"`
	Email        string          `json:"email"`
	UserGroupIDs []string        `json:"user_group_ids"`
	EnvRoles     []inviteEnvRole `json:"env_roles"`
}

type addMemberRequest struct {
	Email string `json:"email"`
}

type updateRolesRequest struct {
	EnvRoles []inviteEnvRole `json:"env_roles"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "test-server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	state := NewState()
	mux := http.NewServeMux()

	// Unauthenticated readiness probe for the CI wait loop (not a Workato endpoint).
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// https://docs.workato.com/workato-api/team.html#list-collaborators
	mux.HandleFunc("GET /api/members", auth(handleListMembers(state)))
	// https://docs.workato.com/workato-api/team.html#add-collaborator
	mux.HandleFunc("POST /api/members", auth(handleAddMember(state)))
	// https://docs.workato.com/workato-api/team.html#invite-a-collaborator
	mux.HandleFunc("POST /api/member_invitations", auth(handleInvite(state)))
	// https://docs.workato.com/workato-api/team.html#list-collaborator-privileges
	mux.HandleFunc("GET /api/members/{id}/privileges", auth(handlePrivileges(state)))
	// https://docs.workato.com/workato-api/team.html#update-collaborator-roles
	mux.HandleFunc("PUT /api/members/{id}", auth(handleUpdateMember(state)))
	// https://docs.workato.com/workato-api/team.html#remove-a-collaborator
	mux.HandleFunc("DELETE /api/members/{id}", auth(handleDeleteMember(state)))
	// https://docs.workato.com/workato-api/roles.html#list-roles
	mux.HandleFunc("GET /api/roles", auth(handleRoles(state)))
	// https://docs.workato.com/workato-api/environment-roles.html#list-environment-roles
	mux.HandleFunc("GET /api/environment_roles", auth(handleEnvironmentRoles(state)))
	// https://docs.workato.com/workato-api/projects.html#list-projects
	mux.HandleFunc("GET /api/projects", auth(handleProjects(state)))
	// https://docs.workato.com/workato-api/folders.html#list-folders
	mux.HandleFunc("GET /api/folders", auth(handleFolders(state)))

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("workato test-server listening on %s (bearer token %q)", listenAddr, testToken)
	return srv.ListenAndServe()
}

// auth enforces the Bearer scheme the real API requires. A permissive validator
// would hide auth-header bugs that only surface against production.
func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if got != "Bearer "+testToken {
			writeErr(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		next(w, r)
	}
}

func handleListMembers(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		members := s.ListMembers()
		writeJSON(w, http.StatusOK, commonPagination[Collaborator]{Data: members, Total: len(members)})
	}
}

func handleInvite(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req inviteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if strings.TrimSpace(req.Email) == "" {
			writeErr(w, http.StatusBadRequest, "email is required")
			return
		}
		// The real API requires a role/group for the invite to be usable; mirror
		// the 400 the connector's invite-shape fix was made to satisfy.
		if len(req.EnvRoles) == 0 && len(req.UserGroupIDs) == 0 {
			writeErr(w, http.StatusBadRequest, "role_name or env_roles is required")
			return
		}

		// Workato's real invite is async (the collaborator must accept an email
		// before becoming a member). The test server collapses that into
		// immediate membership so CI can exercise create -> sync -> delete; the
		// connector's pending-invite (ActionRequired) path is covered against the
		// live API instead.
		roles := make([]SimpleRole, 0, len(req.EnvRoles))
		for _, er := range req.EnvRoles {
			roles = append(roles, SimpleRole{EnvironmentType: er.EnvironmentType, RoleName: er.Name, RoleType: er.RoleType})
		}

		if _, exists := s.CreateMember(req.Name, req.Email, roles); exists {
			writeErr(w, http.StatusConflict, "collaborator already exists")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"result": "ok"})
	}
}

func handleAddMember(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if strings.TrimSpace(req.Email) == "" {
			writeErr(w, http.StatusBadRequest, "email is required")
			return
		}
		if _, exists := s.CreateMember(req.Email, req.Email, nil); exists {
			writeErr(w, http.StatusConflict, "collaborator already exists")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"result": "ok"})
	}
}

func handlePrivileges(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := pathID(r); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid member id")
			return
		}
		// Seeded/created members carry no per-collaborator privilege rows; the
		// connector treats an empty list as "no privileges" and skips them.
		writeJSON(w, http.StatusOK, commonPagination[CollaboratorPrivilege]{Data: []CollaboratorPrivilege{}, Total: 0})
	}
}

// CollaboratorPrivilege mirrors the privileges endpoint element shape.
// https://docs.workato.com/workato-api/team.html#list-collaborator-privileges
type CollaboratorPrivilege struct {
	EnvironmentType string              `json:"environment_type"`
	Name            string              `json:"name"`
	Privileges      map[string][]string `json:"privileges"`
	FolderIDs       []int               `json:"folder_ids"`
}

func handleUpdateMember(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid member id")
			return
		}
		var req updateRolesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		roles := make([]SimpleRole, 0, len(req.EnvRoles))
		for _, er := range req.EnvRoles {
			roles = append(roles, SimpleRole{EnvironmentType: er.EnvironmentType, RoleName: er.Name, RoleType: er.RoleType})
		}
		if ok := s.UpdateMemberRoles(id, roles); !ok {
			writeErr(w, http.StatusNotFound, "collaborator not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"result": "ok"})
	}
}

func handleDeleteMember(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid member id")
			return
		}
		if ok := s.DeleteMember(id); !ok {
			// Real API returns 404 for an unknown member; the connector maps that
			// to NotFound and treats delete as idempotent success.
			writeErr(w, http.StatusNotFound, "collaborator not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleRoles(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roles := s.Roles()
		writeJSON(w, http.StatusOK, paginate(r, roles))
	}
}

// handleEnvironmentRoles serves GET /api/environment_roles. It supports the ?name=
// exact-match filter the connector uses to resolve an invite role's type, and
// page[number]/page[size] pagination for environment-role sync. The response is the
// {data, total} envelope the client decodes into CommonPagination.
func handleEnvironmentRoles(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roles := s.EnvironmentRoles()
		if name := r.URL.Query().Get("name"); name != "" {
			filtered := roles[:0:0]
			for _, role := range roles {
				if role.Name == name {
					filtered = append(filtered, role)
				}
			}
			roles = filtered
		}
		roles = paginateNumbered(r, roles)
		writeJSON(w, http.StatusOK, commonPagination[EnvironmentRole]{Data: roles, Total: len(roles)})
	}
}

func handleProjects(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects := s.Projects()
		writeJSON(w, http.StatusOK, paginate(r, projects))
	}
}

func handleFolders(s *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders := s.Folders()
		if raw := r.URL.Query().Get("parent_id"); raw != "" {
			if parentID, err := strconv.Atoi(raw); err == nil {
				filtered := folders[:0:0]
				for _, f := range folders {
					if f.ParentId == parentID {
						filtered = append(filtered, f)
					}
				}
				folders = filtered
			}
		}
		writeJSON(w, http.StatusOK, paginate(r, folders))
	}
}

// paginate mirrors the page/per_page contract of the roles/projects/folders
// endpoints: the connector requests page 0 and stops once a page returns fewer
// than per_page rows, so any page beyond the first is empty here.
func paginate[T any](r *http.Request, all []T) []T {
	page := 0
	if raw := r.URL.Query().Get("page"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil {
			page = p
		}
	}
	if page > 0 {
		return []T{}
	}
	return all
}

// paginateNumbered mirrors the page[number]/page[size] contract of the
// environment-roles endpoint: the connector requests page 1 and keeps paging until
// a page comes back empty, so any page beyond the first returns no rows here.
func paginateNumbered[T any](r *http.Request, all []T) []T {
	page := 1
	if raw := r.URL.Query().Get("page[number]"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil {
			page = p
		}
	}
	if page > 1 {
		return []T{}
	}
	return all
}

func pathID(r *http.Request) (int, error) {
	return strconv.Atoi(r.PathValue("id"))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeErr emits the Workato `{ "message": "..." }` error envelope the connector
// decodes into client.ApiError.
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"message": msg})
}
