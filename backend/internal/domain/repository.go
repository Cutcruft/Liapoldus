package domain

import "context"

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=repository.go -destination ../../tests/unit/mocks/repository.go -package mocks --self_package github.com/liapoldus/liapoldus/backend

// Storage aggregates every repository the backend exposes so that a single
// store instance can back all application services.
type Storage interface {
	SiteRepository
	PageRepository
	SnapshotRepository
	ContentRepository
	AssetRepository
	RouteRepository
	FormRepository
	ComponentDefinitionRepository
	BuildRepository
	DependencyRepository
	DepPackageRepository
}

type SiteRepository interface {
	CreateSite(context.Context, Site) error
	GetSite(context.Context, string) (Site, error)
	GetSiteBySlug(context.Context, string) (Site, error)
	ListSites(context.Context) ([]Site, error)
	UpdateSite(context.Context, Site) error
	DeleteSite(context.Context, string) error
}

type PageRepository interface {
	CreatePage(context.Context, Page, PageVersion) error
	GetPage(context.Context, string) (Page, error)
	ListPagesBySite(context.Context, string) ([]Page, error)
	UpdatePage(context.Context, Page, PageVersion) error
	ListPageVersions(context.Context, string) ([]PageVersion, error)
	GetPageVersion(context.Context, string, string) (PageVersion, error)
	DeletePage(context.Context, string) error
}

type SnapshotRepository interface {
	CreateSnapshot(context.Context, Snapshot) error
	GetSnapshot(context.Context, string) (Snapshot, error)
	ListSnapshotsBySite(context.Context, string) ([]Snapshot, error)
	DeleteSnapshot(context.Context, string) error
}

type ContentRepository interface {
	CreateContent(context.Context, Content) error
	GetContent(context.Context, string) (Content, error)
	ListContentsBySite(context.Context, string, string) ([]Content, error)
	GetContentsByIDs(context.Context, string, []string) (map[string]Content, error)
	UpdateContent(context.Context, Content) error
	DeleteContent(context.Context, string) error
}

type AssetRepository interface {
	CreateAsset(context.Context, Asset) error
	GetAsset(context.Context, string) (Asset, error)
	ListAssetsBySite(context.Context, string) ([]Asset, error)
	DeleteAsset(context.Context, string) error
}

type RouteRepository interface {
	CreateRoute(context.Context, Route) error
	GetRoute(context.Context, string, string) (Route, error)
	ListRoutesBySite(context.Context, string) ([]Route, error)
	UpdateRoute(context.Context, Route) error
	DeleteRoute(context.Context, string, string) error
}

type FormRepository interface {
	CreateForm(context.Context, Form) error
	GetForm(context.Context, string, string) (Form, error)
	ListFormsBySite(context.Context, string) ([]Form, error)
	UpdateForm(context.Context, Form) error
	DeleteForm(context.Context, string, string) error
	CreateSubmission(context.Context, Submission) error
	ListSubmissionsByForm(context.Context, string, string) ([]Submission, error)
}

type ComponentDefinitionRepository interface {
	Save(context.Context, *ComponentDefinition) error
	Get(context.Context, string, string) (*ComponentDefinition, error)
	List(context.Context, string) ([]ComponentDefinition, error)
	Delete(context.Context, string, string) error
}

type BuildRepository interface {
	CreateBuild(context.Context, Build) error
	GetBuild(context.Context, string) (Build, error)
	ListBuildsBySite(context.Context, string) ([]Build, error)
	// GetBuildBySnapshot returns the most recent Build for a (site, environment,
	// snapshot) triple, or ErrNotFound when none exists. Used for idempotent
	// re-publication of a snapshot.
	GetBuildBySnapshot(context.Context, string, string, string) (Build, error)
	UpdateBuild(context.Context, Build) error
}

// DependencyRepository persists top-level, per-site npm dependencies. The
// (site_id, name) pair is unique; re-declaring a dependency updates its spec.
type DependencyRepository interface {
	CreateDependency(context.Context, Dependency) error
	GetDependency(context.Context, string, string) (Dependency, error)
	ListDependenciesBySite(context.Context, string) ([]Dependency, error)
	UpdateDependency(context.Context, Dependency) error
	DeleteDependency(context.Context, string, string) error
}

// DepPackageRepository is the immutable content-addressed cache of registry
// metadata keyed by (name, version). CreateDepPackage is a no-op when the
// entry already exists (versions are immutable).
type DepPackageRepository interface {
	GetDepPackage(context.Context, string, string) (DepPackage, error)
	CreateDepPackage(context.Context, DepPackage) error
}
