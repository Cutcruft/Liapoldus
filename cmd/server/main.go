package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liapoldus/liapoldus/backend/httpapi"
	"github.com/liapoldus/liapoldus/backend/page"
	"github.com/liapoldus/liapoldus/backend/site"
	"github.com/liapoldus/liapoldus/backend/snapshot"
	"github.com/liapoldus/liapoldus/backend/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	db := store.NewMemory()
	sites := site.NewService(db)
	pages := page.NewService(db, db)
	snapshots := snapshot.NewService(db, db, db)
	server := &http.Server{Addr: env("LIAPOLDUS_ADDR", ":8080"), Handler: httpapi.NewHandler(sites, pages, snapshots, logger), ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		logger.Info("server started", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
