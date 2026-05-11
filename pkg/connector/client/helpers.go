package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

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

func (c *WorkatoClient) doRequest(ctx context.Context, method string, urlAddress *url.URL, res interface{}, body interface{}) error {
	var (
		resp *http.Response
		err  error
	)

	reqOpts := []uhttp.RequestOption{uhttp.WithBearerToken(c.apiKey)}
	if body != nil {
		reqOpts = append(reqOpts, uhttp.WithJSONBody(body))
	}

	req, err := c.httpClient.NewRequest(
		ctx,
		method,
		urlAddress,
		reqOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request object: %w", err)
	}

	var options []uhttp.DoOption

	if res != nil {
		options = append(options, uhttp.WithResponse(&res), uhttp.WithErrorResponse(&ApiError{}))
	}

	resp, err = c.httpClient.Do(req, options...)
	if err != nil {
		return err
	}
	if resp == nil {
		return uhttp.WrapErrors(codes.Internal, "baton-workato doRequest: response is nil with no error, this should never happen")
	}

	defer resp.Body.Close()

	return nil
}

func nextToken[T any](c *WorkatoClient, response []T, page int) string {
	token := ""

	if len(response) == c.pageLimit {
		token = fmt.Sprintf("%d", page+1)
	}

	return token
}
