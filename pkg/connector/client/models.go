package client

import (
	"encoding/json"
	"fmt"
	"time"
)

type apiErrorItem struct {
	Title string `json:"title"`
}

type ApiError struct {
	Errors  []apiErrorItem `json:"errors"`
	Msg     *string        `json:"message"`
	rawBody string
}

func (e *ApiError) UnmarshalJSON(data []byte) error {
	e.rawBody = string(data)
	type plain struct {
		Errors []apiErrorItem `json:"errors"`
		Msg    *string        `json:"message"`
	}
	var p plain
	if err := json.Unmarshal(data, &p); err == nil {
		e.Errors = p.Errors
		e.Msg = p.Msg
	}
	// Unmarshal error intentionally ignored: rawBody is always captured above as fallback,
	// so Message() can surface the raw API response when structured parsing fails.
	return nil
}

// Message implements the error interface.
func (e *ApiError) Message() string {
	if len(e.Errors) > 0 && e.Errors[0].Title != "" {
		return e.Errors[0].Title
	}
	if e.Msg != nil && *e.Msg != "" {
		return fmt.Sprintf("message: %s", *e.Msg)
	}
	if e.rawBody != "" {
		return e.rawBody
	}
	return "unknown error"
}

type CommonPagination[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
}

type SimpleRole struct {
	EnvironmentType string `json:"environment_type"`
	RoleName        string `json:"role_name"`
	RoleType        string `json:"role_type"`
}

type EnvironmentRole struct {
	Id           int       `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	MembersCount int       `json:"members_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *SimpleRole) Equals(other SimpleRole) bool {
	return s.EnvironmentType == other.EnvironmentType &&
		s.RoleName == other.RoleName
}

// InviteEnvRole is a single per-environment role assignment in a member
// invitation. Name is the role NAME (e.g. "Admin"/"Operator"/"Analyst"), not a
// numeric id, matching the official Workato Team API. RoleType is "environment"
// for environment-type roles; when empty Workato defaults it to "privilege_group".
// https://docs.workato.com/workato-api/team.html#invite-a-collaborator
type InviteEnvRole struct {
	EnvironmentType string `json:"environment_type"`
	Name            string `json:"name"`
	RoleType        string `json:"role_type,omitempty"`
}

// InviteCollaboratorRequest is the body for POST /api/member_invitations. It sends
// a Workato invitation email to a brand-new collaborator. env_roles is an array of
// {environment_type, name} objects and user_group_ids is an array of string ids,
// mirroring UpdateCollaboratorRoles and the official Workato Team API.
type InviteCollaboratorRequest struct {
	Name         string          `json:"name"`
	Email        string          `json:"email"`
	UserGroupIDs []string        `json:"user_group_ids,omitempty"`
	EnvRoles     []InviteEnvRole `json:"env_roles,omitempty"`
}

// AddCollaboratorRequest is the body for POST /api/members. It adds an email that
// already belongs to a Workato user directly to the team (no invitation email).
type AddCollaboratorRequest struct {
	Email string `json:"email"`
}

type Collaborator struct {
	Id              int          `json:"id"`
	GrantType       string       `json:"grant_type"`
	Roles           []SimpleRole `json:"roles"`
	LastActivityLog struct {
		Id        int       `json:"id"`
		EventType string    `json:"event_type"`
		CreatedAt time.Time `json:"created_at"`
	} `json:"last_activity_log"`
	ExternalId string    `json:"external_id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	TimeZone   string    `json:"time_zone"`
	CreatedAt  time.Time `json:"created_at"`
}

type CollaboratorPrivilege struct {
	EnvironmentType string              `json:"environment_type"`
	Name            string              `json:"name"`
	Privileges      map[string][]string `json:"privileges"`
	FolderIDs       []int               `json:"folder_ids"`
}

func (c *CollaboratorPrivilege) SimpleRole() SimpleRole {
	return SimpleRole{
		EnvironmentType: c.EnvironmentType,
		RoleName:        c.Name,
	}
}

type Role struct {
	Id          int                 `json:"id"`
	Name        string              `json:"name"`
	Inheritable bool                `json:"inheritable"`
	FolderIDs   []int               `json:"folder_ids"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Privileges  map[string][]string `json:"privileges"`
}

type Folder struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
	ParentId  int       `json:"parent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Project struct {
	Id          int    `json:"id"`
	Description string `json:"description"`
	FolderId    int    `json:"folder_id"`
	Name        string `json:"name"`
}
