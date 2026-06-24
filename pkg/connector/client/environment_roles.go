package client

import (
	"context"
	"net/http"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/annotations"
)

// GetEnvironmentRoleByName fetches an environment role by name using the ?name= filter.
// GET /api/environment_roles?name=... — https://docs.workato.com/workato-api/environment-roles.html
// Required permission: Manage team members (API key scope).
func (c *WorkatoClient) GetEnvironmentRoleByName(ctx context.Context, name string) (*EnvironmentRole, annotations.Annotations, error) {
	var response CommonPagination[*EnvironmentRole]
	uri := c.getPath(environmentRolesPath)
	query := uri.Query()
	query.Add("name", name)
	uri.RawQuery = query.Encode()
	annos, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, annos, err
	}
	for _, role := range response.Data {
		if role != nil && role.Name == name {
			return role, annos, nil
		}
	}
	return nil, annos, nil
}

// GetEnvironmentRole fetches a single environment role by ID.
// GET /api/environment_roles/:id — https://docs.workato.com/workato-api/environment-roles.html
// Required permission: Manage team members (API key scope).
func (c *WorkatoClient) GetEnvironmentRole(ctx context.Context, id string) (*EnvironmentRole, annotations.Annotations, error) {
	var response struct {
		Data EnvironmentRole `json:"data"`
	}
	uri := c.getPath(environmentRolesPath).JoinPath(id)
	annos, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, annos, err
	}
	return &response.Data, annos, nil
}

// GetEnvironmentRoles returns a paginated list of environment roles.
// GET /api/environment_roles — https://docs.workato.com/workato-api/environment-roles.html
// Uses 1-based page[number] + page[size] pagination parameters.
// Required permission: Manage team members (API key scope).
func (c *WorkatoClient) GetEnvironmentRoles(ctx context.Context, pToken string) ([]*EnvironmentRole, string, annotations.Annotations, error) {
	var response CommonPagination[*EnvironmentRole]

	page, err := parsePageToken(pToken, 1)
	if err != nil {
		return nil, "", nil, err
	}

	uri := c.getPath(environmentRolesPath)
	query := uri.Query()
	query.Add("page[number]", strconv.Itoa(page))
	query.Add("page[size]", "100")
	uri.RawQuery = query.Encode()

	annos, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", annos, err
	}

	return response.Data, nextToken(response.Data, page), annos, nil
}
