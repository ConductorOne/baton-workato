package client

import (
	"context"
	"net/http"
)

func (c *WorkatoClient) GetCollaborators(ctx context.Context) ([]Collaborator, error) {
	var response CommonPagination[Collaborator]

	err := c.doRequest(ctx, http.MethodGet, c.getPath(GetCollaboratorsPath), &response, nil)
	if err != nil {
		return nil, err
	}

	return response.Data, nil
}
