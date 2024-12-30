package client

import (
	"context"
	"fmt"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"net/http"
	"strconv"
)

func (c *WorkatoClient) GetFolders(ctx context.Context, parentId *int, pToken *pagination.Token) ([]Folder, string, error) {
	var response []Folder
	var err error

	page := 0
	if pToken.Token != "" {
		page, err = strconv.Atoi(pToken.Token)
		if err != nil {
			return nil, "", ErrInvalidPaginationToken
		}
	}

	uri := c.getPath(GetFoldersPaths)

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

	return response, fmt.Sprintf("%d", page+1), nil
}
