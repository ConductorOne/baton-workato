package client

import (
	"errors"
	"testing"
)

func TestNextToken(t *testing.T) {
	const perPage = defaultPageSize

	tests := []struct {
		name     string
		pageSize int
		perPage  int
		page     int
		want     string
	}{
		{
			name:     "full page yields next page token",
			pageSize: perPage,
			perPage:  perPage,
			page:     1,
			want:     "2",
		},
		{
			name:     "partial page is the last page",
			pageSize: perPage - 1,
			perPage:  perPage,
			page:     1,
			want:     "",
		},
		{
			name:     "empty page is the last page",
			pageSize: 0,
			perPage:  perPage,
			page:     5,
			want:     "",
		},
		{
			name:     "full page mid-pagination advances",
			pageSize: perPage,
			perPage:  perPage,
			page:     7,
			want:     "8",
		},
		{
			// Guards against an infinite loop if a caller ever passes a
			// non-positive page size: terminate instead of advancing forever.
			name:     "non-positive per_page terminates",
			pageSize: perPage,
			perPage:  0,
			page:     1,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := make([]int, tt.pageSize)
			got := nextToken(response, tt.page, tt.perPage)
			if got != tt.want {
				t.Fatalf("nextToken(len=%d, page=%d, perPage=%d) = %q, want %q",
					tt.pageSize, tt.page, tt.perPage, got, tt.want)
			}
		})
	}
}

func TestParsePageToken(t *testing.T) {
	t.Run("empty token returns default page", func(t *testing.T) {
		got, err := parsePageToken("", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 1 {
			t.Fatalf("got %d, want 1", got)
		}
	})

	t.Run("valid token is parsed", func(t *testing.T) {
		got, err := parsePageToken("4", 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 4 {
			t.Fatalf("got %d, want 4", got)
		}
	})

	t.Run("invalid token errors", func(t *testing.T) {
		_, err := parsePageToken("not-a-number", 1)
		if !errors.Is(err, ErrInvalidPaginationToken) {
			t.Fatalf("got %v, want ErrInvalidPaginationToken", err)
		}
	})
}
