package domain

import "context"

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
