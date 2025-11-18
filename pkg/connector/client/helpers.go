package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
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

// httpToGRPCCode maps HTTP status codes to gRPC codes.
// Note: HTTP 429 maps to codes.Unavailable (not ResourceExhausted) to align with baton-sdk conventions.
func httpToGRPCCode(statusCode int) codes.Code {
	switch statusCode {
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return codes.Unavailable
	case http.StatusInternalServerError:
		return codes.Unavailable
	default:
		return codes.Unknown
	}
}

func getError(ctx context.Context, originalErr error, resp *http.Response) error {
	grpcCode := httpToGRPCCode(resp.StatusCode)

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// We expect the response body to be JSON, according to the Workato API docs, but this is not guaranteed.
		l := ctxzap.Extract(ctx)
		l.Debug("failed to read response body", zap.String("body", string(bytes)))
		return uhttp.WrapErrors(grpcCode, fmt.Sprintf("baton-workato getError: failed to read error response body from Workato API: %v", err), originalErr)
	}

	var cErr ApiError
	err = json.Unmarshal(bytes, &cErr)
	if err != nil {
		return uhttp.WrapErrors(grpcCode, fmt.Sprintf("baton-workato getError: failed to parse JSON error response from Workato API: %v", err), originalErr)
	}

	return uhttp.WrapErrors(grpcCode, fmt.Sprintf("baton-workato: API error (status %d): %s", resp.StatusCode, cErr.Message), originalErr)
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
		return fmt.Errorf("failed to create HTTP request object: %w", err)
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

		return uhttp.WrapErrors(codes.Internal, "baton-workato doRequest: response is nil with no error, this should never happen")
	}

	defer resp.Body.Close()

	// Handle supported API errors https://docs.workato.com/en/workato-api.html#http-response-codes
	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return getError(ctx, err, resp)
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
