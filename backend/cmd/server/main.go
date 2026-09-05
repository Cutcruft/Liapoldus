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

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/api/client"
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
	storage, closeStore, err := openStore(startupContext, cfg, logger)
	if err != nil {
		return err
	}
	defer closeStore()

	blobs, err := store.NewDiskBlobStore(cfg.AssetDir)
	if err != nil {
		return err
	}

	sites := domain.NewSiteService(storage)
	pages := domain.NewPageService(storage, storage)
	snapshots := domain.NewSnapshotService(storage, storage, storage)
	contents := domain.NewContentService(storage)
	assets := domain.NewAssetService(storage, blobs, storage)
	routes := domain.NewRouteService(storage)
	forms := domain.NewFormService(storage, storage)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return serve(ctx, cfg, sites, pages, snapshots, contents, assets, routes, forms, logger)
}

func openStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (store.Storage, func(), error) {
	switch cfg.Storage {
	case config.StorageMemory:
		logger.Info("memory storage in use")
		return store.NewMemory(), func() {}, nil
	case config.StoragePostgres:
		postgres, err := store.NewPostgres(ctx, cfg.DatabaseURL)
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

func serve(ctx context.Context, cfg config.Config, sites *domain.SiteService, pages *domain.PageService, snapshots *domain.SnapshotService, contents *domain.ContentService, assets *domain.AssetService, routes *domain.RouteService, forms *domain.FormService, logger *slog.Logger) error {
	adminHandler := admin.NewRouter(admin.App{
		Sites:      sites,
		Pages:      pages,
		Contents:   contents,
		Assets:     assets,
		Routes:     routes,
		Forms:      forms,
		Snapshots:  snapshots,
		Logger:     logger,
		AdminToken: cfg.AdminToken,
	})
	clientHandler := client.NewRouter(&client.App{
		Sites:       sites,
		Contents:    contents,
		Assets:      assets,
		Routes:      routes,
		Forms:       forms,
		Logger:      logger,
		DefaultSlug: cfg.ClientDefaultSlug,
	})

	adminServer := &http.Server{Addr: cfg.AdminAddr, Handler: adminHandler, ReadHeaderTimeout: 5 * time.Second}
	clientServer := &http.Server{Addr: cfg.ClientAddr, Handler: clientHandler, ReadHeaderTimeout: 5 * time.Second}

	errCh := make(chan error, 2)
	go serveListener(adminServer, logger, errCh)
	go serveListener(clientServer, logger, errCh)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
