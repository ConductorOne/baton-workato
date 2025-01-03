package client

import "time"

type ApiError struct {
	Message string `json:"message"`
}

type CommonPagination[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
}

type SimpleRole struct {
	EnvironmentType string `json:"environment_type"`
	RoleName        string `json:"role_name"`
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
