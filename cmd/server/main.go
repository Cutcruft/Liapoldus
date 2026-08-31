package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liapoldus/liapoldus/backend/config"
	"github.com/liapoldus/liapoldus/backend/domain"
	"github.com/liapoldus/liapoldus/backend/esb"
	"github.com/liapoldus/liapoldus/backend/grpcapi"
	"github.com/liapoldus/liapoldus/backend/httpapi"
	"github.com/liapoldus/liapoldus/backend/page"
	"github.com/liapoldus/liapoldus/backend/site"
	"github.com/liapoldus/liapoldus/backend/snapshot"
	"github.com/liapoldus/liapoldus/backend/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStartup()
	siteRepository, pageRepository, snapshotRepository, closeStore, err := openStore(startupContext, cfg, logger)
	if err != nil {
		logger.Error("unable to open storage", "driver", cfg.Storage, "error", err)
		os.Exit(1)
	}
	defer closeStore()
	sites := site.NewService(siteRepository)
	pages := page.NewService(pageRepository, siteRepository)
	snapshots := snapshot.NewService(siteRepository, pageRepository, snapshotRepository)
	extensions := esb.NewRegistry()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.ManagementTransport {
	case config.TransportREST:
		serveREST(ctx, cfg, sites, pages, snapshots, logger)
	case config.TransportGRPC:
		serveGRPC(ctx, cfg, sites, pages, snapshots, extensions, logger)
	}
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

func serveREST(ctx context.Context, cfg config.Config, sites *site.Service, pages *page.Service, snapshots *snapshot.Service, logger *slog.Logger) {
	server := &http.Server{Addr: cfg.Addr, Handler: httpapi.NewHandler(sites, pages, snapshots, logger), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("management server started", "transport", config.TransportREST, "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("management server stopped", "transport", config.TransportREST, "error", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func serveGRPC(ctx context.Context, cfg config.Config, sites *site.Service, pages *page.Service, snapshots *snapshot.Service, extensions *esb.Registry, logger *slog.Logger) {
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		logger.Error("unable to listen", "transport", config.TransportGRPC, "address", cfg.Addr, "error", err)
		os.Exit(1)
	}
	server := grpcapi.NewServer(sites, pages, snapshots, logger, extensions)
	go func() {
		logger.Info("management server started", "transport", config.TransportGRPC, "address", cfg.Addr)
		if err := server.Serve(listener); err != nil {
			logger.Error("management server stopped", "transport", config.TransportGRPC, "error", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	server.GracefulStop()
}
