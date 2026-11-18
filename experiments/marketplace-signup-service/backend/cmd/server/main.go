// Command server runs the sign-up backend.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/auth"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/config"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/httpapi"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/identity"
	"github.com/openshift-online/gcp-hcp/experiments/marketplace-signup-service/backend/internal/workflow"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	policy, err := identity.NewPolicy(cfg.AllowedEmailDomains)
	if err != nil {
		return err
	}

	activator, err := workflow.NewClient(context.Background(), cfg.WorkflowProjectID, cfg.WorkflowRegion, cfg.WorkflowName)
	if err != nil {
		return err
	}

	handler := httpapi.NewHandler(cfg, logger, httpapi.Deps{
		Verifier:  auth.NewVerifier(cfg.OAuthClientID),
		Activator: activator,
	}, policy)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Activations wait on the workflow (bounded by its own 3-minute
		// timeout), so the write deadline must exceed that.
		WriteTimeout: 4 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "port", cfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		logger.Info("shutting down")
		return srv.Shutdown(shutdownCtx)
	}
}
