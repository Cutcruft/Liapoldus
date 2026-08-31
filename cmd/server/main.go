package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liapoldus/liapoldus/backend/config"
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

	db := store.NewMemory()
	sites := site.NewService(db)
	pages := page.NewService(db, db)
	snapshots := snapshot.NewService(db, db, db)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.ManagementTransport {
	case config.TransportREST:
		serveREST(ctx, cfg, sites, pages, snapshots, logger)
	case config.TransportGRPC:
		serveGRPC(ctx, cfg, sites, pages, snapshots, logger)
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

func serveGRPC(ctx context.Context, cfg config.Config, sites *site.Service, pages *page.Service, snapshots *snapshot.Service, logger *slog.Logger) {
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		logger.Error("unable to listen", "transport", config.TransportGRPC, "address", cfg.Addr, "error", err)
		os.Exit(1)
	}
	server := grpcapi.NewServer(sites, pages, snapshots, logger)
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
