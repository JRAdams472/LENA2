package bff

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestExtractBearer(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		want    string
		wantErr bool
	}{
		{name: "valid", header: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "missing header", header: "", wantErr: true},
		{name: "wrong scheme", header: "Basic abc", wantErr: true},
		{name: "empty token", header: "Bearer ", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/graphql", nil)
			if err != nil {
				t.Fatalf("unexpected error building request: %v", err)
			}
			if tc.header != "" {
				req.Header.Set(echo.HeaderAuthorization, tc.header)
			}

			got, err := extractBearer(req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	list := []string{"https://accounts.google.com", "https://example.com"}
	if !contains(list, "https://accounts.google.com") {
		t.Fatal("expected list to contain issuer")
	}
	if contains(list, "https://evil.example.com") {
		t.Fatal("expected list to not contain unknown issuer")
	}
}

func TestContainsAny(t *testing.T) {
	allowed := []string{"client-a", "client-b"}
	if !containsAny(allowed, []string{"client-c", "client-b"}) {
		t.Fatal("expected match on client-b")
	}
	if containsAny(allowed, []string{"client-c", "client-d"}) {
		t.Fatal("expected no match")
	}
}
