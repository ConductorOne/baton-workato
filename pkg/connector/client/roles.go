package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

func (c *WorkatoClient) GetRoles(ctx context.Context, pToken string) ([]Role, string, *v2.RateLimitDescription, error) {
	var response []Role
	var err error

	page := 0
	if pToken != "" {
		page, err = strconv.Atoi(pToken)
		if err != nil {
			return nil, "", nil, errors.Join(ErrInvalidPaginationToken, err)
		}
	}

	uri := c.getPath(GetRolesPath)

	query := uri.Query()
	query.Add("page", fmt.Sprintf("%d", page))
	uri.RawQuery = query.Encode()

	rl, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", rl, err
	}

	return response, nextToken(response, page), rl, nil
}
