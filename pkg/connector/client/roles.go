package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/conductorone/baton-sdk/pkg/annotations"
)

// GetRoles returns a paginated list of custom roles.
// GET /api/roles — https://docs.workato.com/workato-api/roles.html
// Uses 1-based page pagination parameter (Workato's documented default is 1).
// Required permission: Manage team members (API key scope).
func (c *WorkatoClient) GetRoles(ctx context.Context, pToken string) ([]*Role, string, annotations.Annotations, error) {
	var response []*Role

	page, err := parsePageToken(pToken, 1)
	if err != nil {
		return nil, "", nil, err
	}

	uri := c.getPath(rolesPath)
	query := uri.Query()
	query.Add("page", fmt.Sprintf("%d", page))
	query.Add("per_page", fmt.Sprintf("%d", defaultPageSize))
	uri.RawQuery = query.Encode()

	annos, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", annos, err
	}

	return response, nextToken(response, page, defaultPageSize), annos, nil
}
