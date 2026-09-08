// Command lena runs the LENA2 GraphQL BFF API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/JRAdams472/LENA2/internal/analytics"
	"github.com/JRAdams472/LENA2/internal/bff"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/config"
	"github.com/JRAdams472/LENA2/internal/platform/logger"
	"github.com/JRAdams472/LENA2/internal/platform/postgres"
	"github.com/JRAdams472/LENA2/internal/platform/telemetry"
	"github.com/JRAdams472/LENA2/internal/recipe"
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

	log := logger.New(cfg.LogLevel)

	// The 10s timeout applies only to pool creation; a deferred cancel at
	// run scope would keep the context alive for the process lifetime.
	pool, err := func() (*pgxpool.Pool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return postgres.NewPool(ctx, cfg.DatabaseURL)
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
	analyticsSvc := analytics.NewService(pool)
	grocerySvc := grocery.NewService(pool)
	inventorySvc := inventory.NewService(pool)
	mealPlanSvc := mealplan.NewService(pool)
	recipeSvc := recipe.NewService(pool)
	userPrefsSvc := userprefs.NewService(pool)
	wineSvc := wine.NewService(pool)

	authenticator := bff.NewAuthenticator(bff.AuthConfig{
		Issuers:     splitAndTrim(cfg.AuthIssuers),
		Audiences:   splitAndTrim(cfg.AuthAudiences),
		AdminEmails: splitAndTrim(cfg.AdminEmails),
	}, identitySvc)

	e := echo.New()
	e.HideBanner = true
	// The API sits behind Caddy, which appends X-Forwarded-For. Echo's
	// default trust set (loopback and private ranges) matches the compose
	// topology where the only reachable peer is the Caddy container on the
	// internal docker network — port 8080 is not published to the host.
	e.IPExtractor = echo.ExtractIPFromXFFHeader()
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

	resolver := bff.NewResolver(analyticsSvc, grocerySvc, inventorySvc, mealPlanSvc, recipeSvc, userPrefsSvc, wineSvc, identitySvc)
	handler, err := bff.NewGraphQLHandler(resolver,
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
