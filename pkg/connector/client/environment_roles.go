package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

func (c *WorkatoClient) GetEnvironmentRole(ctx context.Context, id string) (*EnvironmentRole, *v2.RateLimitDescription, error) {
	var response struct {
		Data EnvironmentRole `json:"data"`
	}
	uri := c.getPath(fmt.Sprintf("%s/%s", GetEnvironmentRolesPath, id))
	rl, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, rl, err
	}
	return &response.Data, rl, nil
}

func (c *WorkatoClient) GetEnvironmentRoles(ctx context.Context, pToken string) ([]EnvironmentRole, string, *v2.RateLimitDescription, error) {
	var response CommonPagination[EnvironmentRole]
	var err error

	// GET /api/environment_roles uses 1-based page[number] + page[size] params.
	page := 1
	if pToken != "" {
		page, err = strconv.Atoi(pToken)
		if err != nil {
			return nil, "", nil, errors.Join(ErrInvalidPaginationToken, err)
		}
	}

	uri := c.getPath(GetEnvironmentRolesPath)

	query := uri.Query()
	query.Add("page[number]", fmt.Sprintf("%d", page))
	query.Add("page[size]", "100")
	uri.RawQuery = query.Encode()

	rl, err := c.doRequest(ctx, http.MethodGet, uri, &response, nil)
	if err != nil {
		return nil, "", rl, err
	}

	var next string
	if len(response.Data) > 0 {
		next = strconv.Itoa(page + 1)
	}
	return response.Data, next, rl, nil
}
