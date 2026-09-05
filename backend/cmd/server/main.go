package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/config"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStartup()
	siteRepository, pageRepository, snapshotRepository, closeStore, err := openStore(startupContext, cfg, logger)
	if err != nil {
		return err
	}
	defer closeStore()

	sites := domain.NewSiteService(siteRepository)
	pages := domain.NewPageService(pageRepository, siteRepository)
	snapshots := domain.NewSnapshotService(siteRepository, pageRepository, snapshotRepository)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return serveREST(ctx, cfg, sites, pages, snapshots, logger)
}

func openStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (domain.SiteRepository, domain.PageRepository, domain.SnapshotRepository, func(), error) {
	switch cfg.Storage {
	case config.StorageMemory:
		memory := store.NewMemory()
		return memory, memory, memory, func() {}, nil
	case config.StoragePostgres:
		postgres, err := store.NewPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, nil, func() {}, err
		}
		if err := postgres.Migrate(ctx); err != nil {
			postgres.Close()
			return nil, nil, nil, func() {}, err
		}
		logger.Info("postgres storage ready")
		return postgres, postgres, postgres, postgres.Close, nil
	default:
		return nil, nil, nil, func() {}, fmt.Errorf("unsupported storage driver %q", cfg.Storage)
	}
}

func serveREST(ctx context.Context, cfg config.Config, sites *domain.SiteService, pages *domain.PageService, snapshots *domain.SnapshotService, logger *slog.Logger) error {
	server := &http.Server{Addr: cfg.Addr, Handler: httpapi.NewHandler(sites, pages, snapshots, logger), ReadHeaderTimeout: 5 * time.Second}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("management server started", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("rest server: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
