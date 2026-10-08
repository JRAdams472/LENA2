package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/JRAdams472/LENA2/internal/bff"
	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/notifier"
	"github.com/JRAdams472/LENA2/internal/platform/config"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/platform/logger"
	"github.com/JRAdams472/LENA2/internal/session"
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

	// Origins are trimmed: "a, b" must allow both, not silently drop the
	// whitespace-prefixed second origin.
	explicit := buildCORSConfig("https://app.example.com, https://admin.example.com")
	if !explicit.AllowCredentials {
		t.Error("explicit allowlist should allow credentials")
	}
	want := []string{"https://app.example.com", "https://admin.example.com"}
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

func TestHealthcheck(t *testing.T) {
	port := func(srv *httptest.Server) string {
		return srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	}

	t.Run("ready 200 exits 0", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		t.Setenv("LENA_PORT", port(srv))
		if got := healthcheck(); got != 0 {
			t.Errorf("healthcheck() = %d, want 0", got)
		}
	})

	t.Run("ready 503 exits 1", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()
		t.Setenv("LENA_PORT", port(srv))
		if got := healthcheck(); got != 1 {
			t.Errorf("healthcheck() = %d, want 1", got)
		}
	})

	t.Run("connection refused exits 1", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		p := port(srv)
		srv.Close() // nothing listening on p now
		t.Setenv("LENA_PORT", p)
		if got := healthcheck(); got != 1 {
			t.Errorf("healthcheck() = %d, want 1", got)
		}
	})
}

func TestCannedMockHandler(t *testing.T) {
	t.Run("JSON mode returns empty envelopes", func(t *testing.T) {
		resp, err := cannedMockHandler(llm.Request{JSONMode: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resp.Message.Content, `"suggestions":[]`) {
			t.Errorf("JSON reply = %q, want empty envelopes", resp.Message.Content)
		}
	})

	t.Run("after tool result returns canned answer", func(t *testing.T) {
		resp, err := cannedMockHandler(llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hi"},
			{Role: llm.RoleTool, Content: "{}"},
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resp.Message.Content, "mock provider") {
			t.Errorf("reply = %q, want canned tool-result answer", resp.Message.Content)
		}
	})

	t.Run("first turn with tools issues one tool call", func(t *testing.T) {
		resp, err := cannedMockHandler(llm.Request{
			Messages: []llm.Message{{Role: llm.RoleUser, Content: "what expires?"}},
			Tools:    []llm.ToolSpec{{Name: "get_expiring_items"}},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Message.ToolCalls) != 1 || resp.Message.ToolCalls[0].Name != "get_expiring_items" {
			t.Errorf("tool calls = %+v, want one get_expiring_items call", resp.Message.ToolCalls)
		}
	})

	t.Run("no tools returns canned greeting", func(t *testing.T) {
		resp, err := cannedMockHandler(llm.Request{
			Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(resp.Message.Content, "canned demo answer") {
			t.Errorf("reply = %q, want canned greeting", resp.Message.Content)
		}
	})
}

func TestBuildIPExtractor(t *testing.T) {
	extract := buildIPExtractor([]string{"10.0.0.0/8"})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	if ip := extract(req); ip != "203.0.113.9" {
		t.Errorf("trusted proxy XFF extract = %q, want client IP", ip)
	}

	// An invalid CIDR is skipped with a warning instead of aborting.
	extract = buildIPExtractor([]string{"not-a-cidr"})
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:55"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	if ip := extract(req); ip != "192.0.2.1" {
		t.Errorf("untrusted peer should not be able to spoof IP, got %q", ip)
	}
}

// lazyPool returns a pool that parses but never dials — wire tests only
// need construction-time behavior.
func lazyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://127.0.0.1:1/lena_test")
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestNewDomainServicesFlags(t *testing.T) {
	pool := lazyPool(t)
	cfg := config.Config{
		InstacartAPIKey:    "key",
		InstacartBaseURL:   "http://instacart.test",
		IdempotencyEnabled: true,
	}
	s := newDomainServices(cfg, pool)
	t.Cleanup(s.NotifierSvc.Stop)
	t.Cleanup(s.AnalyticsSvc.Stop)

	if s.ShoppingClient == nil {
		t.Error("InstacartAPIKey set should wire a ShoppingClient")
	}
	if s.IdempotencyStore == nil {
		t.Error("IdempotencyEnabled should wire the dedup store")
	}
	if s.RecipeSvc == nil || s.WineSvc == nil || s.OCRClient == nil {
		t.Error("domain services missing")
	}
}

func TestWireAIMockProvider(t *testing.T) {
	pool := lazyPool(t)
	cfg := config.Config{AIProvider: "mock", AIModel: "m", AIEmbedModel: "emb"}
	s := newDomainServices(cfg, pool)
	t.Cleanup(s.NotifierSvc.Stop)
	t.Cleanup(s.AnalyticsSvc.Stop)

	s.wireAI(cfg, pool, logger.New("error"))
	// The recipeembed service is behind the bff.RecipeEmbedder interface —
	// type-assert to stop its background worker.
	if emb, ok := s.RecipeEmbedder.(interface{ Stop() }); ok {
		t.Cleanup(emb.Stop)
	}
	if s.AISvc == nil || s.RecipeImportSvc == nil {
		t.Error("wireAI should build AI + recipe-import services")
	}
	if s.RecipeEmbedder == nil {
		t.Error("mock provider should wire the mock embedder")
	}
}

func TestWirePush(t *testing.T) {
	t.Run("unknown provider errors", func(t *testing.T) {
		s := &serverServices{}
		err := s.wirePush(config.Config{PushProvider: "bogus"})
		if err == nil || !strings.Contains(err.Error(), "unknown LENA_PUSH_PROVIDER") {
			t.Errorf("wirePush(bogus) err = %v, want unknown-provider error", err)
		}
	})

	t.Run("fcm without credentials file errors", func(t *testing.T) {
		s := &serverServices{}
		err := s.wirePush(config.Config{PushProvider: "fcm", FCMCredentialsFile: "no/such/file.json"}) // #nosec G101 -- not a credential, a path that must not exist
		if err == nil || !strings.Contains(err.Error(), "fcm sender init") {
			t.Errorf("wirePush(fcm missing creds) err = %v, want init error", err)
		}
	})

	t.Run("log provider attaches the dev sender", func(t *testing.T) {
		s := &serverServices{NotifierSvc: notifier.NewService(lazyPool(t), notifier.Config{})}
		t.Cleanup(s.NotifierSvc.Stop)
		if err := s.wirePush(config.Config{PushProvider: "log"}); err != nil {
			t.Errorf("wirePush(log) err = %v", err)
		}
	})
}

func TestWireAuth(t *testing.T) {
	newServices := func(t *testing.T) *serverServices {
		pool := lazyPool(t)
		return &serverServices{
			IdentitySvc:  identity.NewService(pool),
			HouseholdSvc: household.NewService(pool),
		}
	}
	log := logger.New("error")

	t.Run("unsupported microsoft tenant warns and continues", func(t *testing.T) {
		s := newServices(t)
		cfg := config.Config{MicrosoftClientID: "id", MicrosoftTenant: "bogus"}
		if err := s.wireAuth(cfg, lazyPool(t), log); err != nil {
			t.Fatalf("wireAuth err = %v", err)
		}
		if s.Authenticator == nil || s.SessionSvc == nil {
			t.Error("wireAuth should still build the authenticator")
		}
	})

	t.Run("consumers tenant trusts the microsoft issuer", func(t *testing.T) {
		s := newServices(t)
		cfg := config.Config{
			MicrosoftClientID: "id", MicrosoftTenant: "consumers",
			FacebookClientID: "fb",
		}
		if err := s.wireAuth(cfg, lazyPool(t), log); err != nil {
			t.Fatalf("wireAuth err = %v", err)
		}
	})
}

func TestRegisterAuthRoutes(t *testing.T) {
	pool := lazyPool(t)
	s := &serverServices{
		IdentitySvc:  identity.NewService(pool),
		HouseholdSvc: household.NewService(pool),
		SessionSvc:   session.NewService(pool, session.Config{}),
	}
	auth, err := bff.NewAuthenticator(bff.AuthConfig{
		Issuers:   []string{"https://issuer.test"},
		Audiences: []string{"aud"},
	}, s.IdentitySvc, s.HouseholdSvc)
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}
	s.Authenticator = auth

	cfg := config.Config{
		DiscordClientID: "d", DiscordClientSecret: "ds", DiscordRedirectURI: "http://x/discord",
		MicrosoftTenant: "consumers", MicrosoftClientID: "m",
		MicrosoftClientSecret: "ms", MicrosoftRedirectURI: "http://x/ms",
		FacebookClientID: "f", FacebookClientSecret: "fs", FacebookRedirectURI: "http://x/fb",
	}
	e := echo.New()
	registerAuthRoutes(e, cfg, s)

	var paths []string
	for _, r := range e.Routes() {
		paths = append(paths, r.Method+" "+r.Path)
	}
	// All providers share a single :provider route; the codeVerifier
	// construction branches above are what this test exercises.
	for _, want := range []string{"POST /auth/session/:provider", "GET /auth/identities", "POST /auth/link", "DELETE /auth/link"} {
		found := false
		for _, p := range paths {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Errorf("route %q not registered; got %v", want, paths)
		}
	}
}
