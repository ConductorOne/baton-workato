package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"google.golang.org/grpc/codes"
)

var (
	AuthHeaderName = "Authorization"

	// WorkatoDataCenters
	// https://docs.workato.com/workato-api.html#base-url
	WorkatoDataCenters = map[string]string{
		"us":      "https://www.workato.com",
		"eu":      "https://app.eu.workato.com",
		"jp":      "https://app.jp.workato.com",
		"sg":      "https://app.sg.workato.com",
		"au":      "https://app.au.workato.com",
		"il":      "https://app.il.workato.com",
		"sandbox": "https://app.trial.workato.com",
	}
)

func (c *WorkatoClient) getPath(path string) *url.URL {
	return c.baseUrl.JoinPath(path)
}

func (c *WorkatoClient) doRequest(ctx context.Context, method string, urlAddress *url.URL, res any, body any) (annotations.Annotations, error) {
	var resp *http.Response
	var rateLimitData v2.RateLimitDescription

	reqOpts := []uhttp.RequestOption{uhttp.WithBearerToken(c.apiKey)}
	if body != nil {
		reqOpts = append(reqOpts, uhttp.WithJSONBody(body))
	}

	req, err := c.httpClient.NewRequest(ctx, method, urlAddress, reqOpts...)
	if err != nil {
		return nil, fmt.Errorf("baton-workato: failed to create HTTP request: %w", err)
	}

	doOpts := []uhttp.DoOption{
		uhttp.WithErrorResponse(&ApiError{}),
		uhttp.WithRatelimitData(&rateLimitData),
	}
	if res != nil {
		doOpts = append(doOpts, uhttp.WithResponse(&res))
	}

	resp, err = c.httpClient.Do(req, doOpts...)
	var annos annotations.Annotations
	annos.WithRateLimiting(&rateLimitData)
	if err != nil {
		return annos, err
	}
	if resp == nil {
		return annos, uhttp.WrapErrors(codes.Internal, "baton-workato doRequest: response is nil with no error, this should never happen")
	}

	defer resp.Body.Close()

	return annos, nil
}

func nextToken[T any](response []T, page int) string {
	if len(response) == 0 {
		return ""
	}
	return fmt.Sprintf("%d", page+1)
}

func parsePageToken(pToken string, defaultPage int) (int, error) {
	if pToken == "" {
		return defaultPage, nil
	}
	page, err := strconv.Atoi(pToken)
	if err != nil {
		return 0, errors.Join(ErrInvalidPaginationToken, err)
	}
	return page, nil
}
