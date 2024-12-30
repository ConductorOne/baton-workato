package client

import (
	"context"
	"fmt"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"net/http"
	"strconv"
)

func (c *WorkatoClient) GetRoles(ctx context.Context, pToken *pagination.Token) ([]Role, string, error) {
	var response []Role
	var err error

	page := 0
	if pToken.Token != "" {
		page, err = strconv.Atoi(pToken.Token)
		if err != nil {
			return nil, "", ErrInvalidPaginationToken
		}
	}

	uri := c.getPath(GetRolesPaths)

	query := uri.Query()
	query.Add("per_page", fmt.Sprintf("%d", c.pageLimit))
	query.Add("page", fmt.Sprintf("%d", page))
	uri.RawQuery = query.Encode()

	err = c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", err
	}

	return response, fmt.Sprintf("%d", page+1), nil
}
