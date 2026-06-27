package connector

import (
	"errors"
	"fmt"
	"testing"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsEnvironmentRolesAccessDenied(t *testing.T) {
	// uhttpUnauthorized mirrors the error the client returns for a 401: uhttp
	// joins a gRPC status error (codes.Unauthenticated) with the parsed
	// ApiError. This is the exact shape behind the reported failure
	// "failed to get environment role ReadOnly: ... 401 Unauthorized".
	uhttpUnauthorized := uhttp.WrapErrors(codes.Unauthenticated, "401 Unauthorized", errors.New("message: Unauthorized"))
	uhttpForbidden := uhttp.WrapErrors(codes.PermissionDenied, "403 Forbidden", errors.New("message: Forbidden"))

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "uhttp joined 401 is access denied",
			err:  uhttpUnauthorized,
			want: true,
		},
		{
			name: "uhttp joined 401 wrapped with %w is still access denied",
			err:  fmt.Errorf("baton-workato: failed to get environment role %s: %w", "ReadOnly", uhttpUnauthorized),
			want: true,
		},
		{
			name: "uhttp joined 403 is access denied",
			err:  uhttpForbidden,
			want: true,
		},
		{
			name: "bare Unauthenticated status is access denied",
			err:  status.Error(codes.Unauthenticated, "nope"),
			want: true,
		},
		{
			name: "not found is not access denied",
			err:  status.Error(codes.NotFound, "missing"),
			want: false,
		},
		{
			name: "rate limited is not access denied",
			err:  status.Error(codes.Unavailable, "slow down"),
			want: false,
		},
		{
			name: "plain error is not access denied",
			err:  errors.New("boom"),
			want: false,
		},
		{
			name: "nil error is not access denied",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEnvironmentRolesAccessDenied(tt.err); got != tt.want {
				t.Fatalf("isEnvironmentRolesAccessDenied(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
