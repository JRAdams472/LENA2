package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/JRAdams472/LENA2/internal/bff"
	"github.com/JRAdams472/LENA2/internal/grocery"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/mealplan"
	"github.com/JRAdams472/LENA2/internal/platform/config"
	"github.com/JRAdams472/LENA2/internal/platform/logger"
	"github.com/JRAdams472/LENA2/internal/platform/postgres"
	"github.com/JRAdams472/LENA2/internal/recipe"
	"github.com/JRAdams472/LENA2/internal/userprefs"
	"github.com/JRAdams472/LENA2/internal/wine"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)

	// The 10s timeout applies only to pool creation; a deferred cancel at
	// main scope would keep the context alive for the process lifetime.
	pool, err := func() (*pgxpool.Pool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return postgres.NewPool(ctx, cfg.DatabaseURL)
	}()
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	e := newServer(*cfg, pool, log)

	go func() {
		addr := ":" + cfg.Port
		log.Info("starting server", "addr", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}
}

func newServer(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) *echo.Echo {
	identitySvc := identity.NewService(pool)
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
	e.Use(middleware.Recover())
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
			if v.Error == nil {
				log.Info("request",
					"request_id", requestID,
					"method", v.Method,
					"uri", v.URI,
					"status", v.Status,
					"latency_ms", v.Latency.Milliseconds(),
				)
			} else {
				log.Error("request",
					"request_id", requestID,
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
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not ready", "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	resolver := bff.NewResolver(grocerySvc, inventorySvc, mealPlanSvc, recipeSvc, userPrefsSvc, wineSvc)
	e.POST("/graphql", bff.NewGraphQLHandler(resolver), authenticator.Middleware())
	return e
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
		cfg.AllowOriginFunc = func(origin string) (bool, error) { return true, nil }
		cfg.AllowCredentials = false
	} else {
		cfg.AllowOrigins = strings.Split(allowedOrigins, ",")
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
