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

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/api/client"
	"github.com/liapoldus/liapoldus/backend/internal/application"
	"github.com/liapoldus/liapoldus/backend/internal/config"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/db"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
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

	startupContext, cancelStartup := context.WithTimeout(context.Background(), cfg.StartupTimeout)
	defer cancelStartup()
	store, closeStore, err := openStore(startupContext, cfg, logger)
	if err != nil {
		return err
	}
	defer closeStore()

	blobs, err := storage.NewDiskBlobStore(cfg.AssetDir)
	if err != nil {
		return err
	}

	services := application.New(store, blobs, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return serve(ctx, cfg, services, logger)
}

func openStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (domain.Storage, func(), error) {
	switch cfg.Storage {
	case config.StorageMemory:
		logger.Info("memory storage in use")
		return storage.NewMemory(), func() {}, nil
	case config.StoragePostgres:
		postgres, err := db.NewPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, func() {}, err
		}
		if err := postgres.Migrate(ctx); err != nil {
			postgres.Close()
			return nil, func() {}, err
		}
		logger.Info("postgres storage ready")
		return postgres, postgres.Close, nil
	default:
		return nil, func() {}, fmt.Errorf("unsupported storage driver %q", cfg.Storage)
	}
}

func serve(ctx context.Context, cfg config.Config, services *application.Services, logger *slog.Logger) error {
	adminHandler := admin.NewRouter(admin.App{
		Sites:      services.Sites,
		Pages:      services.Pages,
		Contents:   services.Contents,
		Assets:     services.Assets,
		Routes:     services.Routes,
		Forms:      services.Forms,
		Snapshots:  services.Snapshots,
		Logger:     logger,
		AdminToken: cfg.AdminToken,
	})
	clientHandler := client.NewRouter(&client.App{
		Sites:                   services.Sites,
		Contents:                services.Contents,
		Assets:                  services.Assets,
		Routes:                  services.Routes,
		Forms:                   services.Forms,
		Logger:                  logger,
		DefaultSlug:             cfg.ClientDefaultSlug,
		DefaultRedirectStatus:   cfg.RedirectDefaultStatus,
		AssetCacheMaxAgeSeconds: cfg.AssetCacheMaxAgeSeconds,
	})

	adminServer := &http.Server{Addr: cfg.AdminAddr, Handler: adminHandler, ReadHeaderTimeout: cfg.ReadHeaderTimeout}
	clientServer := &http.Server{Addr: cfg.ClientAddr, Handler: clientHandler, ReadHeaderTimeout: cfg.ReadHeaderTimeout}

	errCh := make(chan error, 2)
	go serveListener(adminServer, logger, errCh)
	go serveListener(clientServer, logger, errCh)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		_ = adminServer.Shutdown(shutdownCtx)
		_ = clientServer.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

func serveListener(server *http.Server, logger *slog.Logger, errCh chan<- error) {
	logger.Info("server started", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errCh <- fmt.Errorf("server %s: %w", server.Addr, err)
		return
	}
	errCh <- nil
}
