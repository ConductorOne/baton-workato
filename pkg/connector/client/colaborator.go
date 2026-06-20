package client

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetCollaborators returns all team collaborators.
// GET /api/members — https://docs.workato.com/workato-api/team.html
// The Workato API returns all members in a single response; pagination is not supported.
// Required permission: Manage team members (API key scope).
func (c *WorkatoClient) GetCollaborators(ctx context.Context) ([]*Collaborator, annotations.Annotations, error) {
	var response CommonPagination[*Collaborator]

	annos, err := c.doRequest(ctx, http.MethodGet, c.getPath(collaboratorsPath), &response, nil)
	if err != nil {
		return nil, annos, err
	}

	return response.Data, annos, nil
}

// GetCollaboratorPrivileges returns the role privileges assigned to a collaborator.
// GET /api/members/:id/privileges — https://docs.workato.com/workato-api/team.html
// The Workato API returns all privileges in a single response; pagination is not supported.
// Required permission: Manage team members (API key scope).
func (c *WorkatoClient) GetCollaboratorPrivileges(ctx context.Context, id int) ([]*CollaboratorPrivilege, annotations.Annotations, error) {
	var response CommonPagination[*CollaboratorPrivilege]

	uri := c.getPath(collaboratorsPath).JoinPath(strconv.Itoa(id), "privileges")
	annos, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, annos, err
	}

	if len(response.Data) == 0 {
		return nil, annos, uhttp.WrapErrors(codes.NotFound, "baton-workato: no collaborator privileges found")
	}

	return response.Data, annos, nil
}

// GetCollaboratorByEmail returns the collaborator matching email, or a NotFound
// status error if no collaborator in the tenant has that email. The Workato email
// filter is a substring (contains) match, so we apply an exact case-insensitive
// check against the returned results.
func (c *WorkatoClient) GetCollaboratorByEmail(ctx context.Context, email string) (*Collaborator, error) {
	var response CommonPagination[Collaborator]

	uri := c.getPath(collaboratorsPath)
	query := uri.Query()
	query.Set("email", email)
	uri.RawQuery = query.Encode()

	_, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, err
	}

	for i := range response.Data {
		if strings.EqualFold(response.Data[i].Email, email) {
			return &response.Data[i], nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "baton-workato: collaborator with email %s not found", email)
}

// InviteCollaborator sends a Workato invitation email to a new collaborator via
// POST /api/member_invitations. The collaborator becomes a full member only once
// they accept the email invitation.
func (c *WorkatoClient) InviteCollaborator(ctx context.Context, body InviteCollaboratorRequest) error {
	_, err := c.doRequest(ctx, http.MethodPost, c.getPath(InviteCollaboratorPath), nil, body)
	return err
}

// AddExistingCollaborator adds an email that already belongs to a Workato user
// directly to the team via POST /api/members (no invitation email is sent).
func (c *WorkatoClient) AddExistingCollaborator(ctx context.Context, email string) error {
	body := AddCollaboratorRequest{Email: email}
	_, err := c.doRequest(ctx, http.MethodPost, c.getPath(AddCollaboratorPath), nil, body)
	return err
}

// DeleteCollaborator removes a collaborator via DELETE /api/members/{id}. Workato
// has no soft-disable, so this is a hard delete. Returns a NotFound status error
// (mapped from HTTP 404) when the collaborator is already gone.
func (c *WorkatoClient) DeleteCollaborator(ctx context.Context, id int) error {
	_, err := c.doRequest(ctx, http.MethodDelete, c.getPath(collaboratorsPath).JoinPath(strconv.Itoa(id)), nil, nil)
	return err
}

// UpdateCollaboratorRoles updates the collaborator's roles for the environments
// included in env_roles. Roles in environments not included are not affected.
// PUT /api/members/:id — https://docs.workato.com/workato-api/team.html
// Required permission: Manage team members (API key scope).
func (c *WorkatoClient) UpdateCollaboratorRoles(ctx context.Context, id int, roles []SimpleRole) (annotations.Annotations, error) {
	// The JSON payload field names differ from the SimpleRole struct tags; see API docs.
	type SimpleRoleRequest struct {
		EnvironmentType string `json:"environment_type"`
		RoleName        string `json:"name"`
		RoleType        string `json:"role_type,omitempty"`
	}

	var rolesRequest []SimpleRoleRequest
	for _, role := range roles {
		rolesRequest = append(rolesRequest, SimpleRoleRequest(role))
	}

	body := struct {
		EnvRoles []SimpleRoleRequest `json:"env_roles"`
	}{
		EnvRoles: rolesRequest,
	}

	return c.doRequest(ctx, http.MethodPut, c.getPath(collaboratorsPath).JoinPath(strconv.Itoa(id)), nil, body)
}
