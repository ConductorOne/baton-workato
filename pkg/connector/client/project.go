package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/conductorone/baton-sdk/pkg/annotations"
)

// GetProjects returns a paginated list of projects.
// GET /api/projects — https://docs.workato.com/workato-api/projects.html
// Uses 0-based page pagination parameter.
// Required permission: Manage team members (API key scope).
func (c *WorkatoClient) GetProjects(ctx context.Context, pToken string) ([]*Project, string, annotations.Annotations, error) {
	var response []*Project

	page, err := parsePageToken(pToken, 0)
	if err != nil {
		return nil, "", nil, err
	}

	uri := c.getPath(projectsPath)
	query := uri.Query()
	query.Add("page", fmt.Sprintf("%d", page))
	uri.RawQuery = query.Encode()

	annos, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", annos, err
	}

	return response, nextToken(response, page), annos, nil
}
