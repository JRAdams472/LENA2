package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/JRAdams472/LENA2/internal/bff"
	"github.com/JRAdams472/LENA2/internal/identity"
	"github.com/JRAdams472/LENA2/internal/inventory"
	"github.com/JRAdams472/LENA2/internal/platform/config"
	"github.com/JRAdams472/LENA2/internal/platform/logger"
	"github.com/JRAdams472/LENA2/internal/platform/postgres"
	"github.com/JRAdams472/LENA2/internal/recipe"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	identitySvc := identity.NewService(pool)
	inventorySvc := inventory.NewService(pool)
	recipeSvc := recipe.NewService(pool)

	authenticator := bff.NewAuthenticator(bff.AuthConfig{
		Issuers:   splitAndTrim(cfg.AuthIssuers),
		Audiences: splitAndTrim(cfg.AuthAudiences),
	}, identitySvc)

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	corsCfg := middleware.CORSConfig{
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}
	if cfg.CORSAllowedOrigins == "*" {
		corsCfg.AllowOriginFunc = func(origin string) (bool, error) { return true, nil }
	} else {
		corsCfg.AllowOrigins = strings.Split(cfg.CORSAllowedOrigins, ",")
	}
	e.Use(middleware.CORSWithConfig(corsCfg))

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	resolver := bff.NewResolver(inventorySvc, recipeSvc)
	e.POST("/graphql", bff.NewGraphQLHandler(resolver), authenticator.Middleware())

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
