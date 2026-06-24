package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"google.golang.org/grpc/codes"
)

var (
	ErrApiKeyIsEmpty          = errors.New("baton-workato: api key is empty")
	ErrInvalidPaginationToken = errors.New("baton-workato: invalid pagination token")
)

const (
	// https://docs.workato.com/workato-api/team.html
	collaboratorsPath = "api/members"

	// https://docs.workato.com/workato-api/roles.html
	rolesPath = "api/roles"

	// https://docs.workato.com/workato-api/environment-roles.html
	environmentRolesPath = "api/environment_roles"

	// https://docs.workato.com/workato-api/projects.html
	projectsPath = "api/projects"

	// https://docs.workato.com/workato-api/folders.html
	foldersPath = "api/folders"
)

type WorkatoClient struct {
	apiKey     string
	baseUrl    *url.URL
	httpClient *uhttp.BaseHttpClient
}

func NewWorkatoClient(ctx context.Context, apiKey, baseUrl string) (*WorkatoClient, error) {
	parseBaseUrl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, uhttp.WrapErrors(codes.InvalidArgument, "baton-workato: failed to parse base URL", err)
	}

	if apiKey == "" {
		return nil, uhttp.WrapErrors(codes.InvalidArgument, "baton-workato: failed to validate API key", ErrApiKeyIsEmpty)
	}

	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, fmt.Errorf("baton-workato: failed to create HTTP client: %w", err)
	}

	uhttpClient, err := uhttp.NewBaseHttpClientWithContext(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("baton-workato: failed to create base HTTP client: %w", err)
	}

	return &WorkatoClient{
		httpClient: uhttpClient,
		baseUrl:    parseBaseUrl,
		apiKey:     apiKey,
	}, nil
}
