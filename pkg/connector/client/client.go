package client

import (
	"context"
	"errors"
	"net/url"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

var (
	ErrApiKeyIsEmpty          = errors.New("baton-workato: api key is empty")
	ErrInvalidPaginationToken = errors.New("baton-workato: invalid pagination token")
)

var (
	GetCollaboratorsPath    = "api/members"
	GetCollaboratorByIdPath = "api/members/%d/privileges"
	GetRolesPaths           = "api/roles"
	GetFoldersPaths         = "api/folders"
)

type WorkatoClient struct {
	apiKey     string
	baseUrl    *url.URL
	httpClient *uhttp.BaseHttpClient
	pageLimit  int
}

func NewWorkatoClient(ctx context.Context, apiKey, baseUrl string) (*WorkatoClient, error) {
	parseBaseUrl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	if apiKey == "" {
		return nil, ErrApiKeyIsEmpty
	}

	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, err
	}

	uhtppClient, err := uhttp.NewBaseHttpClientWithContext(ctx, httpClient)
	if err != nil {
		return nil, err
	}

	return &WorkatoClient{
		httpClient: uhtppClient,
		baseUrl:    parseBaseUrl,
		apiKey:     apiKey,
		pageLimit:  500,
	}, nil
}
