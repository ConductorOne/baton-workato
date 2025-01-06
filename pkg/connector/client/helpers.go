package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
)

var (
	AuthHeaderName = "Authorization"

	// WorkatoDataCenters
	// https://docs.workato.com/workato-api.html#base-url
	WorkatoDataCenters = map[string]string{
		"us": "https://www.workato.com",
		"eu": "https://app.eu.workato.com",
		"jp": "https://app.jp.workato.com",
		"sg": "https://app.sg.workato.com",
		"au": "https://app.au.workato.com",
	}
)

func (c *WorkatoClient) getPath(path string) *url.URL {
	return c.baseUrl.JoinPath(path)
}

func getError(resp *http.Response) (ApiError, error) {
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ApiError{}, err
	}

	var cErr ApiError
	err = json.Unmarshal(bytes, &cErr)
	if err != nil {
		return cErr, err
	}

	return cErr, nil
}

func (c *WorkatoClient) doRequest(ctx context.Context, method string, urlAddress *url.URL, res interface{}, body interface{}) error {
	var (
		resp *http.Response
		err  error
	)

	req, err := c.httpClient.NewRequest(
		ctx,
		method,
		urlAddress,
		uhttp.WithHeader(AuthHeaderName, fmt.Sprintf("Bearer %s", c.apiKey)),
		uhttp.WithJSONBody(body),
	)
	if err != nil {
		return err
	}

	switch method {
	case http.MethodGet:
		resp, err = c.httpClient.Do(req, uhttp.WithResponse(&res))
		if resp != nil {
			defer resp.Body.Close()
		}
	case http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodPut:
		resp, err = c.httpClient.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
	}

	if resp != nil {
		if resp.StatusCode == http.StatusUnauthorized {
			return errors.New("unauthorized")
		}

		if resp.StatusCode == http.StatusForbidden {
			return errors.New("forbidden")
		}

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			cErr, err := getError(resp)
			if err != nil {
				return err
			}

			return errors.New(cErr.Message)
		}

		return err
	}

	if err != nil {
		return err
	}

	return nil
}

func nextToken[T any](c *WorkatoClient, response []T, page int) string {
	token := ""

	if len(response) == c.pageLimit {
		token = fmt.Sprintf("%d", page+1)
	}

	return token
}
