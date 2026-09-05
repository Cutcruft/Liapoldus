package store

import "github.com/liapoldus/liapoldus/backend/internal/domain"

// Storage aggregates every repository the backend exposes so that a single
// store instance can back all domain services.
type Storage interface {
	domain.SiteRepository
	domain.PageRepository
	domain.SnapshotRepository
	domain.ContentRepository
	domain.AssetRepository
	domain.RouteRepository
	domain.FormRepository
}

var (
	_ Storage = (*Memory)(nil)
	_ Storage = (*Postgres)(nil)
)
