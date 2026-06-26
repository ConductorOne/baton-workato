package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/conductorone/baton-sdk/pkg/annotations"
)

// GetFolders returns a paginated list of folders, optionally filtered by parent folder.
// GET /api/folders — https://docs.workato.com/workato-api/folders.html
// Uses 1-based page pagination parameter (Workato's documented default is 1).
// Required permission: Manage team members (API key scope).
func (c *WorkatoClient) GetFolders(ctx context.Context, parentId *int, pToken string) ([]*Folder, string, annotations.Annotations, error) {
	var response []*Folder

	page, err := parsePageToken(pToken, 1)
	if err != nil {
		return nil, "", nil, err
	}

	uri := c.getPath(foldersPath)
	query := uri.Query()
	query.Add("page", fmt.Sprintf("%d", page))
	query.Add("per_page", fmt.Sprintf("%d", defaultPageSize))
	if parentId != nil {
		query.Add("parent_id", fmt.Sprintf("%d", *parentId))
	}
	uri.RawQuery = query.Encode()

	annos, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", annos, err
	}

	return response, nextToken(response, page, defaultPageSize), annos, nil
}
