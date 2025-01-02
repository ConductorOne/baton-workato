package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

func (c *WorkatoClient) GetRoles(ctx context.Context, pToken string) ([]Role, string, error) {
	var response []Role
	var err error

	page := 0
	if pToken != "" {
		page, err = strconv.Atoi(pToken)
		if err != nil {
			return nil, "", ErrInvalidPaginationToken
		}
	}

	uri := c.getPath(GetRolesPath)

	query := uri.Query()
	query.Add("per_page", fmt.Sprintf("%d", c.pageLimit))
	query.Add("page", fmt.Sprintf("%d", page))
	uri.RawQuery = query.Encode()

	err = c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", err
	}

	return response, nextToken(response, page), nil
}
