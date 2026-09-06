package main

import (
	"reflect"
	"testing"
)

func TestBuildCORSConfig(t *testing.T) {
	// Wildcard origins must never be combined with credentials: reflecting
	// any Origin header while allowing credentials is a CSRF/credential-leak
	// vector.
	wild := buildCORSConfig("*")
	if wild.AllowCredentials {
		t.Error("wildcard origins must not allow credentials")
	}
	if wild.AllowOriginFunc == nil {
		t.Error("wildcard origins should reflect via AllowOriginFunc")
	}
	if ok, err := wild.AllowOriginFunc("https://evil.example.com"); err != nil || !ok {
		t.Error("wildcard should accept any origin")
	}

	explicit := buildCORSConfig("https://app.example.com, https://admin.example.com")
	if !explicit.AllowCredentials {
		t.Error("explicit allowlist should allow credentials")
	}
	want := []string{"https://app.example.com", " https://admin.example.com"}
	if !reflect.DeepEqual(explicit.AllowOrigins, want) {
		t.Errorf("AllowOrigins = %v, want %v", explicit.AllowOrigins, want)
	}
}

func TestSplitAndTrim(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a, b, c", []string{"a", "b", "c"}},
		{"a,, b ", []string{"a", "b"}},
		{"", []string{}},
		{"  only  ", []string{"only"}},
	}

	for _, c := range cases {
		got := splitAndTrim(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitAndTrim(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
