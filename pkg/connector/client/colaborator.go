package client

import (
	"context"
	"fmt"
	"net/http"
)

func (c *WorkatoClient) GetCollaborators(ctx context.Context) ([]Collaborator, error) {
	var response CommonPagination[Collaborator]

	err := c.doRequest(ctx, http.MethodGet, c.getPath(GetCollaboratorsPath), &response, nil)
	if err != nil {
		return nil, err
	}

	return response.Data, nil
}

func (c *WorkatoClient) GetCollaboratorPrivileges(ctx context.Context, id int) ([]*CollaboratorPrivilege, error) {
	var response CommonPagination[*CollaboratorPrivilege]

	pathString := fmt.Sprintf(GetCollaboratorByIdPath, id)

	err := c.doRequest(ctx, http.MethodGet, c.getPath(pathString), &response, nil)
	if err != nil {
		return nil, err
	}

	if len(response.Data) != 1 {
		return nil, fmt.Errorf("baton-workato: expected 1 collaborator, got %d", len(response.Data))
	}

	return response.Data, nil
}

func (c *WorkatoClient) UpdateCollaboratorRoles(ctx context.Context, id int, roles []SimpleRole) error {
	pathString := fmt.Sprintf(UpdateCollaboratorByIdPath, id)

	// Needs this because the json payload it's different https://docs.workato.com/workato-api/team.html#update-collaborator-roles
	type SimpleRoleRequest struct {
		EnvironmentType string `json:"environment_type"`
		RoleName        string `json:"name"`
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

	err := c.doRequest(ctx, http.MethodPut, c.getPath(pathString), nil, body)
	if err != nil {
		return err
	}

	return nil
}
