package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

func (c *WorkatoClient) GetFolders(ctx context.Context, parentId *int, pToken string) ([]Folder, string, *v2.RateLimitDescription, error) {
	var response []Folder
	var err error

	page := 0
	if pToken != "" {
		page, err = strconv.Atoi(pToken)
		if err != nil {
			return nil, "", nil, errors.Join(ErrInvalidPaginationToken, err)
		}
	}

	uri := c.getPath(GetFoldersPath)

	query := uri.Query()
	query.Add("page", fmt.Sprintf("%d", page))

	if parentId != nil {
		query.Add("parent_id", fmt.Sprintf("%d", *parentId))
	}

	uri.RawQuery = query.Encode()

	rl, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", rl, err
	}

	return response, nextToken(response, page), rl, nil
}
