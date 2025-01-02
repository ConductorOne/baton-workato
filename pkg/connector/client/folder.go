package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

func (c *WorkatoClient) GetFolders(ctx context.Context, parentId *int, pToken string) ([]Folder, string, error) {
	var response []Folder
	var err error

	page := 0
	if pToken != "" {
		page, err = strconv.Atoi(pToken)
		if err != nil {
			return nil, "", ErrInvalidPaginationToken
		}
	}

	uri := c.getPath(GetFoldersPath)

	query := uri.Query()
	query.Add("per_page", fmt.Sprintf("%d", c.pageLimit))
	query.Add("page", fmt.Sprintf("%d", page))

	if parentId != nil {
		query.Add("parent_id", fmt.Sprintf("%d", *parentId))
	}

	uri.RawQuery = query.Encode()

	err = c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", err
	}

	return response, nextToken(response, page), nil
}
