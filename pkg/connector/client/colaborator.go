package client

import (
	"context"
	"fmt"
	"net/http"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (c *WorkatoClient) GetCollaborators(ctx context.Context) ([]Collaborator, *v2.RateLimitDescription, error) {
	var response CommonPagination[Collaborator]

	rl, err := c.doRequest(ctx, http.MethodGet, c.getPath(GetCollaboratorsPath), &response, nil)
	if err != nil {
		return nil, rl, err
	}

	return response.Data, rl, nil
}

func (c *WorkatoClient) GetCollaboratorPrivileges(ctx context.Context, id int) ([]*CollaboratorPrivilege, *v2.RateLimitDescription, error) {
	var response CommonPagination[*CollaboratorPrivilege]
	pathString := fmt.Sprintf(GetCollaboratorByIdPath, id)

	rl, err := c.doRequest(ctx, http.MethodGet, c.getPath(pathString), &response, nil)
	if err != nil {
		return nil, rl, err
	}

	if len(response.Data) == 0 {
		return nil, rl, status.Errorf(codes.NotFound, "baton-workato: no collaborator privileges found")
	}

	return response.Data, rl, nil
}

func (c *WorkatoClient) UpdateCollaboratorRoles(ctx context.Context, id int, roles []SimpleRole) (*v2.RateLimitDescription, error) {
	pathString := fmt.Sprintf(UpdateCollaboratorByIdPath, id)

	// Needs this because the json payload it's different https://docs.workato.com/workato-api/team.html#update-collaborator-roles
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

	rl, err := c.doRequest(ctx, http.MethodPut, c.getPath(pathString), nil, body)
	if err != nil {
		return rl, err
	}

	return rl, nil
}
