package domain

import "context"

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -package mocks -destination ../../tests/unit/mocks/repository.go github.com/liapoldus/liapoldus/backend/internal/domain SiteRepository,PageRepository,SnapshotRepository

type SiteRepository interface {
	CreateSite(context.Context, Site) error
	GetSite(context.Context, string) (Site, error)
	GetSiteBySlug(context.Context, string) (Site, error)
}

type PageRepository interface {
	CreatePage(context.Context, Page, PageVersion) error
	GetPage(context.Context, string) (Page, error)
	ListPagesBySite(context.Context, string) ([]Page, error)
	UpdatePage(context.Context, Page, PageVersion) error
	ListPageVersions(context.Context, string) ([]PageVersion, error)
}

type SnapshotRepository interface {
	CreateSnapshot(context.Context, Snapshot) error
	GetSnapshot(context.Context, string) (Snapshot, error)
	ListSnapshotsBySite(context.Context, string) ([]Snapshot, error)
}
