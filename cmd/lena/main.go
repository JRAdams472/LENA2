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
	"github.com/JRAdams472/LENA2/internal/app/recipeembed"
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
	"github.com/JRAdams472/LENA2/internal/platform/dbtx"
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
	// The distroless runtime has no shell or curl for the compose
	// healthcheck, so the binary self-probes: `lena -healthcheck` exits 0
	// when /ready answers 200.
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(healthcheck())
	}
	os.Exit(run())
}

func healthcheck() int {
	port := os.Getenv("LENA_PORT")
	if port == "" {
		port = "8080"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The URL is fixed to localhost; only the port comes from the
	// container's own environment.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:"+port+"/ready", nil) // #nosec G704
	if err != nil {
		return 1
	}
	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- see above.
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
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
// serverServices groups the long-lived components newServer wires into
// the resolver and route handlers so each construction stage stays small.
type serverServices struct {
	IdentitySvc      *identity.Service
	AnalyticsSvc     *analytics.Service
	GrocerySvc       *grocery.Service
	HouseholdSvc     *household.Service
	InventorySvc     *inventory.Service
	EventSvc         *event.Service
	MealPlanSvc      *mealplan.Service
	NotifierSvc      *notifier.Service
	RecipeSvc        *recipe.Service
	UserPrefsSvc     *userprefs.Service
	WineSvc          *wine.Service
	OCRClient        *ocrclient.Client
	IdempotencyStore *idempotency.Store
	RecipeImportSvc  *recipeimport.Service
	AISvc            *ai.Service
	RecipeEmbedder   bff.RecipeEmbedder
	SessionSvc       *session.Service
	Authenticator    *bff.Authenticator
}

// newServer builds the Echo instance and GraphQL resolver. A nil tel is
// supported for tests and disables /metrics and HTTP metrics middleware;
// telemetry.Setup never returns (nil, nil) in production.
func newServer(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger, tel *telemetry.Telemetry) (*echo.Echo, *bff.Resolver, error) {
	s := newDomainServices(cfg, pool)
	s.wireAI(cfg, pool, log)
	if err := s.wireAuth(cfg, pool, log); err != nil {
		return nil, nil, err
	}
	if err := s.wirePush(cfg); err != nil {
		return nil, nil, err
	}
	e, err := newEcho(cfg, log, tel, pool, s.Authenticator)
	if err != nil {
		return nil, nil, err
	}
	resolver, err := registerAPIRoutes(e, cfg, pool, s)
	if err != nil {
		return nil, nil, err
	}
	return e, resolver, nil
}

// newDomainServices builds the pool-backed domain services, the stateless
// OCR client, and the dedup store, then starts their background workers.
func newDomainServices(cfg config.Config, pool *pgxpool.Pool) *serverServices {
	s := &serverServices{
		IdentitySvc: identity.NewService(pool).
			WithProtectedEmails(splitAndTrim(cfg.ProtectedEmails)),
		AnalyticsSvc: analytics.NewService(pool, analytics.Config{
			DecayInterval: cfg.AnalyticsDecayInterval,
			HalfLifeDays:  cfg.AnalyticsHalfLifeDays,
		}),
		GrocerySvc:   grocery.NewService(pool),
		HouseholdSvc: household.NewService(pool),
		InventorySvc: inventory.NewService(pool),
		EventSvc:     event.NewService(pool),
		MealPlanSvc:  mealplan.NewService(pool),
		NotifierSvc: notifier.NewService(pool, notifier.Config{
			SweepInterval: cfg.NotificationSweepInterval,
			NotifyHour:    cfg.NotificationHour,
			ExpiryDays:    cfg.NotificationExpiryDays,
		}),
		RecipeSvc:    recipe.NewService(pool),
		UserPrefsSvc: userprefs.NewService(pool),
		WineSvc:      wine.NewService(pool),
		OCRClient:    ocrclient.New(cfg.OCRServiceURL, cfg.OCRTimeout),
	}
	// Opt-outs apply to event-driven notifications too, not just sweep
	// reminders — the gate is checked at write time.
	s.HouseholdSvc.WithNotifyGate(s.NotifierSvc)
	// Reminder sweep runs until Resolver.Shutdown calls Stop.
	s.NotifierSvc.Start(context.Background())
	// Decayed-score rebuild feeds analytics-driven search ranking; same
	// lifecycle as the sweep.
	s.AnalyticsSvc.Start(context.Background())
	// The dedup store is a plain pool-backed component, not a domain
	// service: its writes must commit immediately and never join a
	// request's transaction.
	if cfg.IdempotencyEnabled {
		s.IdempotencyStore = idempotency.NewStore(pool, idempotency.Config{
			KeyTTL:      cfg.IdempotencyKeyTTL,
			AutoTTL:     cfg.IdempotencyAutoTTL,
			InFlightTTL: cfg.IdempotencyInFlightTTL,
			WaitTimeout: cfg.IdempotencyWaitTimeout,
		})
	}
	return s
}

// wireAI configures the OCR→draft import pipeline and the assistant's
// provider, tool registry, and embedding service.
// wirePush selects the push sender and starts the delivery worker, which
// drains household.push_delivery to device tokens until Resolver.Shutdown
// stops it. The default "log" provider records would-be sends so dev and
// e2e exercise the full pipeline credential-free; "fcm" needs the mounted
// service account.
func (s *serverServices) wirePush(cfg config.Config) error {
	dc := notifier.DeliveryConfig{
		PollInterval: cfg.PushPollInterval,
		MaxAttempts:  cfg.PushMaxAttempts,
	}
	switch cfg.PushProvider {
	case "fcm":
		sender, err := notifier.NewFCMSender(context.Background(), cfg.FCMCredentialsFile)
		if err != nil {
			return fmt.Errorf("fcm sender init: %w", err)
		}
		s.NotifierSvc.AttachDelivery(context.Background(), sender, dc)
	case "log", "":
		s.NotifierSvc.AttachDelivery(context.Background(), notifier.LogSender{}, dc)
	default:
		return fmt.Errorf("unknown LENA_PUSH_PROVIDER %q (want log|fcm)", cfg.PushProvider)
	}
	return nil
}

func (s *serverServices) wireAI(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) {

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
	s.RecipeImportSvc = recipeimport.NewService(
		pool,
		s.OCRClient,
		ollamaClient,
		s.InventorySvc,
		s.RecipeSvc,
		profanityDetector,
		recipeimport.ConfigFromPlatform(&cfg),
	)
	// Re-enqueue jobs orphaned by a previous shutdown before serving traffic.
	if err := s.RecipeImportSvc.Start(context.Background()); err != nil {
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
	tools.RegisterPantryTools(aiTools, s.UserPrefsSvc, s.InventorySvc)
	tools.RegisterMealPlanTools(aiTools, s.MealPlanSvc, s.RecipeSvc)
	tools.RegisterRecipeTools(aiTools, s.RecipeSvc, s.InventorySvc)
	tools.RegisterTasteTools(aiTools, s.AnalyticsSvc, s.RecipeSvc)
	tools.RegisterEventTools(aiTools, s.EventSvc, s.RecipeSvc)
	tools.RegisterCellarTools(aiTools, s.UserPrefsSvc, s.WineSvc, s.RecipeSvc, s.InventorySvc)
	tools.RegisterAllergenTools(aiTools, s.RecipeSvc, s.InventorySvc, s.InventorySvc)
	s.AISvc = ai.NewService(aiProvider, aiTools, ai.Config{
		MaxToolRounds: cfg.AIMaxToolRounds,
	})

	// Semantic recipe search: an Ollama URL alone enables embeddings — the
	// index builds even when the chat assistant is off. The mock pair keeps
	// e2e deterministic.
	var embedSvc *recipeembed.Service
	if cfg.AIProvider == "mock" {
		embedSvc = recipeembed.NewService(llm.NewMockEmbedder(), cfg.AIEmbedModel, s.RecipeSvc, s.InventorySvc, recipeembed.WithLogger(log))
	} else if cfg.OllamaURL != "" {
		embedSvc = recipeembed.NewService(
			llm.NewOllamaEmbedder(cfg.OllamaURL, cfg.AIEmbedModel, cfg.AIEmbedTimeout, "", ""),
			cfg.AIEmbedModel, s.RecipeSvc, s.InventorySvc, recipeembed.WithLogger(log))
	}
	// RecipeEmbedder is an interface: assign only when configured so a nil
	// *Service never slips through as a non-nil interface.
	if embedSvc != nil {
		s.RecipeEmbedder = embedSvc
		s.RecipeImportSvc.SetEmbedder(embedSvc)
		// The model only sees the semantic tool when an embedder can
		// actually run it.
		tools.RegisterSemanticSearchTool(aiTools, s.RecipeSvc, embedSvc)
		embedSvc.Start()
	}
}

// wireAuth builds the session service and the OIDC authenticator, adding
// trusted issuers/audiences for the code-exchange providers.
func (s *serverServices) wireAuth(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) error {

	// Refresh-token sessions: an empty secret leaves the service disabled
	// (Enabled() == false), preserving OIDC-only auth.
	s.SessionSvc = session.NewService(pool, session.Config{
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
	}, s.IdentitySvc, s.HouseholdSvc)
	if err != nil {
		return fmt.Errorf("auth config: %w", err)
	}
	authenticator.SetSessions(s.SessionSvc)
	authenticator.SetUnitOfWork(dbtx.NewUnitOfWork(pool))
	s.Authenticator = authenticator
	return nil
}

// newEcho builds the Echo instance with timeouts, middleware, health
// probes, and (when telemetry is enabled) the guarded /metrics route.
func newEcho(cfg config.Config, log *slog.Logger, tel *telemetry.Telemetry, pool *pgxpool.Pool, auth *bff.Authenticator) (*echo.Echo, error) {
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
		LogStatus:   true,
		LogURI:      true,
		LogMethod:   true,
		LogLatency:  true,
		LogRemoteIP: true,
		LogError:    true,
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
					"remote_ip", v.RemoteIP,
					"method", v.Method,
					"uri", v.URI,
					"status", v.Status,
					"latency_ms", v.Latency.Milliseconds(),
				)
			} else {
				log.Error("request",
					"request_id", requestID,
					"trace_id", traceID,
					"remote_ip", v.RemoteIP,
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
			return nil, fmt.Errorf("http metrics middleware: %w", err)
		}
		e.Use(httpMetrics)
		// /metrics requires the same bearer token as /graphql: unauthenticated
		// exposure would leak operational data. Scrapers must present a token
		// from a trusted issuer.
		e.GET("/metrics", echo.WrapHandler(tel.MetricsHandler()), auth.Middleware())
	}
	return e, nil
}

// registerAPIRoutes mounts the GraphQL endpoint and the auth/session HTTP
// routes, then returns the resolver for lifecycle shutdown.
func registerAPIRoutes(e *echo.Echo, cfg config.Config, pool *pgxpool.Pool, s *serverServices) (*bff.Resolver, error) {
	resolver := bff.NewResolver(pool,
		bff.Services{
			Analytics:      s.AnalyticsSvc,
			Grocery:        s.GrocerySvc,
			Inventory:      s.InventorySvc,
			MealPlan:       s.MealPlanSvc,
			Event:          s.EventSvc,
			Recipe:         s.RecipeSvc,
			UserPrefs:      s.UserPrefsSvc,
			Wine:           s.WineSvc,
			Identity:       s.IdentitySvc,
			RecipeImport:   s.RecipeImportSvc,
			Household:      s.HouseholdSvc,
			Notifier:       s.NotifierSvc,
			Auth:           s.Authenticator,
			AI:             s.AISvc,
			OCR:            s.OCRClient,
			RecipeEmbedder: s.RecipeEmbedder,
		},
		bff.Options{
			NutritionPhotoMaxBytes: cfg.NutritionPhotoMaxBytes,
			RecipeScanMaxBytes:     cfg.RecipeScanMaxBytes,
			Idempotency:            s.IdempotencyStore,
		})
	// Introspection is admin-only: members still get full API operation
	// support, but cannot enumerate the schema (including admin mutations).
	// LENA_GRAPHQL_DISABLE_INTROSPECTION disables it for everyone.
	handler, err := bff.NewGraphQLHandler(resolver,
		cfg.GraphQLTimeout,
		cfg.GraphQLAITimeout,
		cfg.GraphQLMaxCost,
		graphql.MaxDepth(cfg.GraphQLMaxDepth),
		graphql.MaxQueryLength(cfg.GraphQLMaxQueryLength),
		graphql.RestrictIntrospection(bff.AllowIntrospectionForAdmins(cfg.GraphQLDisableIntrospection)))
	if err != nil {
		return nil, err
	}
	// Middleware order matters: the body limit runs first (cheapest drop),
	// then the IP-keyed limiter throttles unauthenticated floods before
	// they reach auth, then the authenticator, then the per-user limiter.
	// An empty body limit disables the middleware (tests and embedders that
	// do not load config defaults); the rate limiters already treat <= 0 as
	// disabled.
	graphqlMW := []echo.MiddlewareFunc{
		bff.IPRateLimiter(cfg.IPRateLimitPerMinute, cfg.IPRateLimitBurst),
		s.Authenticator.Middleware(),
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
	bff.NewSessionHandler(s.SessionSvc).RegisterRoutes(e,
		s.Authenticator.Middleware(),
		bff.IPRateLimiter(cfg.IPRateLimitPerMinute, cfg.IPRateLimitBurst))
	registerAuthRoutes(e, cfg, s)
	return resolver, nil
}

// registerAuthRoutes mounts the provider code-exchange sign-in and the
// account link/unlink endpoints.
func registerAuthRoutes(e *echo.Echo, cfg config.Config, s *serverServices) {
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
			s.Authenticator, false); v != nil {
			codeVerifiers[bff.MicrosoftProvider] = v
		}
	}
	if v := bff.NewOAuthOIDCVerifier(cfg.FacebookClientID, cfg.FacebookClientSecret,
		cfg.FacebookRedirectURI, "https://graph.facebook.com/v21.0/oauth/access_token",
		s.Authenticator, true); v != nil {
		codeVerifiers[bff.FacebookProvider] = v
	}

	// Account linking: list/link/unlink provider identities. Link is a
	// step-up operation — an OIDC bearer is fresh proof, while a session
	// bearer must be accompanied by a fresh provider credential.
	bff.NewLinkHandler(s.Authenticator, codeVerifiers, s.IdentitySvc).RegisterRoutes(e,
		s.Authenticator.Middleware(),
		bff.IPRateLimiter(cfg.IPRateLimitPerMinute, cfg.IPRateLimitBurst))

	// Provider code-exchange sign-in (POST /auth/session/{provider}). The
	// per-IP limiter alone cannot bound a distributed flood because every
	// attempt also makes an outbound call to the provider's token endpoint;
	// the global bucket caps total exchange volume regardless of source.
	bff.NewProviderSessionHandler(codeVerifiers, s.Authenticator, s.SessionSvc).RegisterRoutes(e,
		bff.IPRateLimiter(cfg.IPRateLimitPerMinute, cfg.IPRateLimitBurst),
		bff.GlobalRateLimiter(cfg.AuthCodeExchangeRateLimitPerMinute, cfg.AuthCodeExchangeRateLimitBurst))
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
		Content: "Hi! I'm Dot's canned demo answer — no tools needed for that one.",
	}}, nil
}
