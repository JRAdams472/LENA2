// Command lena runs the LENA2 GraphQL BFF API server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	// otelecho is deprecated in favour of github.com/labstack/echo-opentelemetry,
	// which requires Echo v5; we are on v4, so keep otelecho until that upgrade.
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho" //nolint:staticcheck
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/JRAdams472/LENA2/internal/ai"
	"github.com/JRAdams472/LENA2/internal/ai/tools"
	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/app/recipeimport"
	"github.com/JRAdams472/LENA2/internal/bff"
	"github.com/JRAdams472/LENA2/internal/event"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/household"
	"github.com/JRAdams472/LENA2/internal/idempotency"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/notifier"
	"github.com/JRAdams472/LENA2/internal/platform/config"
	"github.com/JRAdams472/LENA2/internal/platform/llm"
	"github.com/JRAdams472/LENA2/internal/platform/logger"
	"github.com/JRAdams472/LENA2/internal/platform/ocrclient"
	"github.com/JRAdams472/LENA2/internal/platform/postgres"
	"github.com/JRAdams472/LENA2/internal/platform/profanity"
	"github.com/JRAdams472/LENA2/internal/platform/telemetry"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/session"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

func main() {
	os.Exit(run())
}

// run wires configuration, storage, telemetry and HTTP serving, then
// blocks until a signal or a server error. Keeping the logic out of main
// means deferred cleanup (pool close, telemetry flush, analytics drain)
// runs on every exit path — os.Exit would otherwise skip the defers.
func run() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}
	if err := cfg.ValidateServer(); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}

	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)

	// The 10s timeout applies only to pool creation; a deferred cancel at
	// run scope would keep the context alive for the process lifetime.
	pool, err := func() (*pgxpool.Pool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return postgres.NewPool(ctx, cfg.DatabaseURL, cfg.DatabaseStatementTimeout)
	}()
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		return 1
	}
	defer pool.Close()

	// Telemetry setup dials the OTLP endpoint; bound it so a hung collector
	// can't stall startup indefinitely (LENA-050).
	telCtx, telCancel := context.WithTimeout(context.Background(), 10*time.Second)
	tel, err := telemetry.Setup(telCtx, cfg.ServiceName, cfg.OTLPEndpoint, cfg.OTLPInsecure, pool)
	telCancel()
	if err != nil {
		log.Error("telemetry setup failed", "error", err)
		return 1
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tel.Shutdown(ctx)
	}()

	e, resolver, err := newServer(*cfg, pool, log, tel)
	if err != nil {
		log.Error("server setup failed", "error", err)
		return 1
	}

	serverErr := make(chan error, 1)
	go func() {
		addr := ":" + cfg.Port
		log.Info("starting server", "addr", addr)
		if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	code := 0
	select {
	case <-sig:
	case err := <-serverErr:
		log.Error("server error", "error", err)
		code = 1
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}

	// Drain in-flight background work (analytics events, recommendation
	// recomputation) before the pool is closed underneath it.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer drainCancel()
	if err := resolver.Shutdown(drainCtx); err != nil {
		log.Error("background worker drain error", "error", err)
	}

	return code
}

// newServer builds the Echo instance and GraphQL resolver. A nil tel is
// supported for tests and disables /metrics and HTTP metrics middleware;
// telemetry.Setup never returns (nil, nil) in production.
func newServer(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger, tel *telemetry.Telemetry) (*echo.Echo, *bff.Resolver, error) {
	identitySvc := identity.NewService(pool).
		WithProtectedEmails(splitAndTrim(cfg.ProtectedEmails))
	analyticsSvc := analytics.NewService(pool, analytics.Config{
		DecayInterval: cfg.AnalyticsDecayInterval,
		HalfLifeDays:  cfg.AnalyticsHalfLifeDays,
	})
	grocerySvc := grocery.NewService(pool)
	householdSvc := household.NewService(pool)
	inventorySvc := inventory.NewService(pool)
	eventSvc := event.NewService(pool)
	mealPlanSvc := mealplan.NewService(pool)
	notifierSvc := notifier.NewService(pool, notifier.Config{
		SweepInterval: cfg.NotificationSweepInterval,
		NotifyHour:    cfg.NotificationHour,
		ExpiryDays:    cfg.NotificationExpiryDays,
	})
	// Opt-outs apply to event-driven notifications too, not just sweep
	// reminders — the gate is checked at write time.
	householdSvc.WithNotifyGate(notifierSvc)
	// Reminder sweep runs until Resolver.Shutdown calls Stop.
	notifierSvc.Start(context.Background())
	// Decayed-score rebuild feeds analytics-driven search ranking; same
	// lifecycle as the sweep.
	analyticsSvc.Start(context.Background())
	recipeSvc := recipe.NewService(pool)
	userPrefsSvc := userprefs.NewService(pool)
	wineSvc := wine.NewService(pool)
	ocrClient := ocrclient.New(cfg.OCRServiceURL, cfg.OCRTimeout)

	// The dedup store is a plain pool-backed component, not a domain
	// service: its writes must commit immediately and never join a
	// request's transaction.
	var idemStore *idempotency.Store
	if cfg.IdempotencyEnabled {
		idemStore = idempotency.NewStore(pool, idempotency.Config{
			KeyTTL:      cfg.IdempotencyKeyTTL,
			AutoTTL:     cfg.IdempotencyAutoTTL,
			InFlightTTL: cfg.IdempotencyInFlightTTL,
			WaitTimeout: cfg.IdempotencyWaitTimeout,
		})
	}

	// Kept as the interface so a missing URL yields an untyped nil —
	// recipeimport checks `ollama == nil` to disable the draft stage.
	var ollamaClient recipeimport.LLMClient
	if cfg.OllamaURL != "" {
		p, err := llm.NewProvider(llm.Params{
			Provider:    "ollama",
			URL:         cfg.OllamaURL,
			Model:       cfg.OllamaModel,
			Temperature: cfg.OllamaTemperature,
			NumCtx:      cfg.OllamaNumCtx,
		})
		if err != nil {
			log.Error("failed to configure ollama provider", "error", err)
			os.Exit(1)
		}
		ollamaClient = p
	}
	profanityDetector := profanity.New(cfg.ProfanityExtraTerms)
	recipeImportSvc := recipeimport.NewService(
		pool,
		ocrClient,
		ollamaClient,
		inventorySvc,
		recipeSvc,
		profanityDetector,
		recipeimport.ConfigFromPlatform(&cfg),
	)
	// Re-enqueue jobs orphaned by a previous shutdown before serving traffic.
	if err := recipeImportSvc.Start(context.Background()); err != nil {
		log.Warn("recipe import recovery failed", "error", err)
	}

	// AI assistant: a nil provider keeps the service constructed but
	// disabled — aiAvailable reports false and askAssistant returns
	// UNAVAILABLE.
	aiModel := cfg.AIModel
	if aiModel == "" {
		aiModel = cfg.OllamaModel
	}
	aiProvider, err := llm.NewProvider(llm.Params{
		Provider:    cfg.AIProvider,
		URL:         cfg.OllamaURL,
		Model:       aiModel,
		Temperature: cfg.AITemperature,
		NumCtx:      cfg.AINumCtx,
		Timeout:     cfg.AITimeout,
	})
	if err != nil {
		log.Error("failed to configure AI provider", "error", err)
		os.Exit(1)
	}
	// e2e and demos run LENA_AI_PROVIDER=mock; an empty scripted queue would
	// error on the first call, so give the mock a deterministic handler —
	// one canned tool call, then a canned reply; empty results for the
	// JSON-mode suggesters.
	if mp, ok := aiProvider.(*llm.MockProvider); ok {
		mp.Handler = cannedMockHandler
	}
	aiTools := tools.New()
	tools.RegisterPantryTools(aiTools, userPrefsSvc, inventorySvc)
	tools.RegisterMealPlanTools(aiTools, mealPlanSvc, recipeSvc)
	tools.RegisterRecipeTools(aiTools, recipeSvc, inventorySvc)
	tools.RegisterTasteTools(aiTools, analyticsSvc, recipeSvc)
	tools.RegisterEventTools(aiTools, eventSvc, recipeSvc)
	tools.RegisterCellarTools(aiTools, userPrefsSvc, wineSvc, recipeSvc, inventorySvc)
	aiSvc := ai.NewService(aiProvider, aiTools, ai.Config{
		MaxToolRounds: cfg.AIMaxToolRounds,
	})

	// Refresh-token sessions: an empty secret leaves the service disabled
	// (Enabled() == false), preserving OIDC-only auth.
	sessionSvc := session.NewService(pool, session.Config{
		Secret:     cfg.SessionSecret,
		AccessTTL:  cfg.SessionAccessTTL,
		RefreshTTL: cfg.SessionRefreshTTL,
	})

	// Code-exchange OIDC providers (Microsoft, Facebook) trust their own
	// issuer + client id — no duplicate AUTH_ISSUERS/AUTH_AUDIENCES entry
	// is required; explicit config still wins.
	issuers := splitAndTrim(cfg.AuthIssuers)
	audiences := splitAndTrim(cfg.AuthAudiences)
	if cfg.MicrosoftClientID != "" {
		if iss := bff.MicrosoftIssuer(cfg.MicrosoftTenant); iss != "" {
			issuers, audiences = bff.TrustIssuer(issuers, audiences, iss, cfg.MicrosoftClientID)
		} else {
			log.Warn("unsupported MICROSOFT_TENANT (use 'consumers' or a tenant GUID); microsoft sign-in disabled",
				"tenant", cfg.MicrosoftTenant)
		}
	}
	issuers, audiences = bff.TrustIssuer(issuers, audiences, bff.FacebookIssuer(), cfg.FacebookClientID)

	authenticator, err := bff.NewAuthenticator(bff.AuthConfig{
		Issuers:     issuers,
		Audiences:   audiences,
		AdminEmails: splitAndTrim(cfg.AdminEmails),
	}, identitySvc, householdSvc)
	if err != nil {
		return nil, nil, fmt.Errorf("auth config: %w", err)
	}
	authenticator.SetSessions(sessionSvc)

	e := echo.New()
	e.HideBanner = true
	// The API sits behind Caddy, which appends X-Forwarded-For. Only
	// explicitly configured proxy CIDRs (plus loopback) are trusted; the
	// default private range trust is disabled so a direct spoofed XFF
	// cannot change the client IP used by the rate limiter.
	e.IPExtractor = buildIPExtractor(splitAndTrim(cfg.TrustedProxyCIDRs))
	// Bound connection lifecycle: without timeouts the server is exposed
	// to slowloris and large-body resource exhaustion.
	e.Server.ReadHeaderTimeout = cfg.HTTPReadHeaderTimeout
	e.Server.ReadTimeout = cfg.HTTPReadTimeout
	e.Server.WriteTimeout = cfg.HTTPWriteTimeout
	e.Server.IdleTimeout = cfg.HTTPIdleTimeout
	e.Use(middleware.Recover())
	e.Use(otelecho.Middleware(cfg.ServiceName))
	e.Use(middleware.RequestID())

	e.Use(middleware.CORSWithConfig(buildCORSConfig(cfg.CORSAllowedOrigins)))
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:  true,
		LogURI:     true,
		LogMethod:  true,
		LogLatency: true,
		LogError:   true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			requestID := c.Response().Header().Get(echo.HeaderXRequestID)
			var traceID string
			if sc := oteltrace.SpanContextFromContext(c.Request().Context()); sc.IsValid() {
				traceID = sc.TraceID().String()
			}
			if v.Error == nil {
				log.Info("request",
					"request_id", requestID,
					"trace_id", traceID,
					"method", v.Method,
					"uri", v.URI,
					"status", v.Status,
					"latency_ms", v.Latency.Milliseconds(),
				)
			} else {
				log.Error("request",
					"request_id", requestID,
					"trace_id", traceID,
					"method", v.Method,
					"uri", v.URI,
					"status", v.Status,
					"latency_ms", v.Latency.Milliseconds(),
					"error", v.Error,
				)
			}
			return nil
		},
	}))

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.GET("/ready", func(c echo.Context) error {
		if err := pool.Ping(c.Request().Context()); err != nil {
			// Do not leak the database error string to unauthenticated callers.
			log.Error("readiness check failed", "error", err)
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	if tel != nil {
		httpMetrics, err := telemetry.HTTPMetrics()
		if err != nil {
			return nil, nil, fmt.Errorf("http metrics middleware: %w", err)
		}
		e.Use(httpMetrics)
		// /metrics requires the same bearer token as /graphql: unauthenticated
		// exposure would leak operational data. Scrapers must present a token
		// from a trusted issuer.
		e.GET("/metrics", echo.WrapHandler(tel.MetricsHandler()), authenticator.Middleware())
	}

	resolver := bff.NewResolver(pool,
		bff.Services{
			Analytics:    analyticsSvc,
			Grocery:      grocerySvc,
			Inventory:    inventorySvc,
			MealPlan:     mealPlanSvc,
			Event:        eventSvc,
			Recipe:       recipeSvc,
			UserPrefs:    userPrefsSvc,
			Wine:         wineSvc,
			Identity:     identitySvc,
			RecipeImport: recipeImportSvc,
			Household:    householdSvc,
			Notifier:     notifierSvc,
			Auth:         authenticator,
			AI:           aiSvc,
			OCR:          ocrClient,
		},
		bff.Options{
			NutritionPhotoMaxBytes: cfg.NutritionPhotoMaxBytes,
			RecipeScanMaxBytes:     cfg.RecipeScanMaxBytes,
			Idempotency:            idemStore,
		})
	handler, err := bff.NewGraphQLHandler(resolver,
		cfg.GraphQLTimeout,
		cfg.GraphQLMaxCost,
		graphql.MaxDepth(cfg.GraphQLMaxDepth),
		graphql.MaxQueryLength(cfg.GraphQLMaxQueryLength))
	if err != nil {
		return nil, nil, err
	}
	// Middleware order matters: the body limit runs first (cheapest drop),
	// then the IP-keyed limiter throttles unauthenticated floods before
	// they reach auth, then the authenticator, then the per-user limiter.
	// An empty body limit disables the middleware (tests and embedders that
	// do not load config defaults); the rate limiters already treat <= 0 as
	// disabled.
	graphqlMW := []echo.MiddlewareFunc{
		bff.IPRateLimiter(cfg.IPRateLimitPerMinute, cfg.IPRateLimitBurst),
		authenticator.Middleware(),
		bff.GraphQLRateLimiter(cfg.GraphQLRateLimitPerMinute, cfg.GraphQLRateLimitBurst),
	}
	if cfg.GraphQLBodyLimit != "" {
		graphqlMW = append([]echo.MiddlewareFunc{middleware.BodyLimit(cfg.GraphQLBodyLimit)}, graphqlMW...)
	}
	e.POST("/graphql", handler, graphqlMW...)

	// Session endpoints sit outside GraphQL: refresh must work when the
	// access token is already expired, which the /graphql auth middleware
	// would reject. Refresh/revoke are unauthenticated (the refresh token
	// is the credential) and IP-rate-limited; create runs behind auth.
	bff.NewSessionHandler(sessionSvc).RegisterRoutes(e,
		authenticator.Middleware(),
		bff.IPRateLimiter(cfg.IPRateLimitPerMinute, cfg.IPRateLimitBurst))

	// Code-exchange providers: the authorization code is the credential,
	// exchanged server-side (client_secret never leaves the server).
	// Discord is plain OAuth2; Microsoft and Facebook return an OIDC
	// id_token verified by the standard issuer/JWKS path.
	codeVerifiers := map[string]bff.CodeVerifier{}
	if v := bff.NewDiscordVerifier(cfg.DiscordClientID, cfg.DiscordClientSecret, cfg.DiscordRedirectURI); v != nil {
		codeVerifiers[bff.DiscordProvider] = v
	}
	if iss := bff.MicrosoftIssuer(cfg.MicrosoftTenant); iss != "" {
		if v := bff.NewOAuthOIDCVerifier(cfg.MicrosoftClientID, cfg.MicrosoftClientSecret,
			cfg.MicrosoftRedirectURI,
			"https://login.microsoftonline.com/"+cfg.MicrosoftTenant+"/oauth2/v2.0/token",
			authenticator, false); v != nil {
			codeVerifiers[bff.MicrosoftProvider] = v
		}
	}
	if v := bff.NewOAuthOIDCVerifier(cfg.FacebookClientID, cfg.FacebookClientSecret,
		cfg.FacebookRedirectURI, "https://graph.facebook.com/v21.0/oauth/access_token",
		authenticator, true); v != nil {
		codeVerifiers[bff.FacebookProvider] = v
	}

	// Account linking: list/link/unlink provider identities. Link is a
	// step-up operation — an OIDC bearer is fresh proof, while a session
	// bearer must be accompanied by a fresh provider credential.
	bff.NewLinkHandler(authenticator, codeVerifiers, identitySvc).RegisterRoutes(e,
		authenticator.Middleware(),
		bff.IPRateLimiter(cfg.IPRateLimitPerMinute, cfg.IPRateLimitBurst))

	// Provider code-exchange sign-in (POST /auth/session/{provider}).
	bff.NewProviderSessionHandler(codeVerifiers, authenticator, sessionSvc).RegisterRoutes(e,
		bff.IPRateLimiter(cfg.IPRateLimitPerMinute, cfg.IPRateLimitBurst))
	return e, resolver, nil
}

// buildCORSConfig builds the CORS middleware config. When origins are
// wildcarded we reflect any origin but must not allow credentials —
// reflecting arbitrary origins with credentials enabled leaks
// Authorization/cookie data and opens a CSRF vector. Credentials are only
// allowed with an explicit origin allowlist.
func buildCORSConfig(allowedOrigins string) middleware.CORSConfig {
	cfg := middleware.CORSConfig{
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		MaxAge:       86400,
	}
	if allowedOrigins == "*" {
		cfg.AllowOriginFunc = func(string) (bool, error) { return true, nil }
		cfg.AllowCredentials = false
	} else {
		cfg.AllowOrigins = splitAndTrim(allowedOrigins)
		cfg.AllowCredentials = true
	}
	return cfg
}

// splitAndTrim splits a comma-separated env value into a trimmed, non-empty slice.
func splitAndTrim(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// buildIPExtractor configures how the API resolves the "real" client IP
// from X-Forwarded-For. Only the configured proxy CIDRs and loopback are
// trusted; private ranges are not, so a spoofed header from an untrusted
// peer cannot alter the rate-limiter key.
func buildIPExtractor(cidrs []string) echo.IPExtractor {
	opts := []echo.TrustOption{echo.TrustLoopback(true), echo.TrustPrivateNet(false)}
	for _, c := range cidrs {
		_, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			// Misconfiguration should fail early; warn and ignore this CIDR.
			slog.Default().Warn("invalid trusted proxy CIDR, ignoring", "cidr", c, "error", err)
			continue
		}
		opts = append(opts, echo.TrustIPRange(ipnet))
	}
	return echo.ExtractIPFromXFFHeader(opts...)
}

// cannedMockHandler drives LENA_AI_PROVIDER=mock for e2e and demos: the
// first turn requests one real tool (so the UI's "looked up" trace is
// exercised end-to-end), the turn after a tool result returns a fixed
// answer, and JSON-mode calls from the suggesters get every known envelope
// empty.
func cannedMockHandler(req llm.Request) (llm.Response, error) {
	if req.JSONMode {
		return llm.Response{Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: `{"suggestions":[],"pairings":[],"fixes":[]}`,
		}}, nil
	}
	if len(req.Messages) > 0 && req.Messages[len(req.Messages)-1].Role == llm.RoleTool {
		return llm.Response{Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "I checked your household data — based on what's on hand, you're set for the week. (mock provider)",
		}}, nil
	}
	if len(req.Tools) > 0 {
		return llm.Response{Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID:        "call_1",
				Name:      "get_expiring_items",
				Arguments: json.RawMessage(`{"days":7}`),
			}},
		}}, nil
	}
	return llm.Response{Message: llm.Message{
		Role:    llm.RoleAssistant,
		Content: "Hi! I'm LENA's canned demo answer — no tools needed for that one.",
	}}, nil
}
