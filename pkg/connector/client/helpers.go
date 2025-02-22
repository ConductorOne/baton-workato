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

func getError(originalErr error, resp *http.Response) error {
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Join(originalErr, err)
	}

	var cErr ApiError
	err = json.Unmarshal(bytes, &cErr)
	if err != nil {
		return errors.Join(originalErr, err)
	}

	return errors.Join(originalErr, errors.New(cErr.Message))
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
		uhttp.WithBearerToken(c.apiKey),
		uhttp.WithJSONBody(body),
	)
	if err != nil {
		return err
	}

	var options []uhttp.DoOption

	if res != nil {
		options = append(options, uhttp.WithResponse(&res))
	}

	resp, err = c.httpClient.Do(req, options...)

	if resp == nil {
		if err != nil {
			return err
		}

		return errors.New("baton-workato: response is nil and error is nil, this should never happen, might be a bug in the http client")
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return getError(err, resp)
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
