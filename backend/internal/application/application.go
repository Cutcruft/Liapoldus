package application

import (
	"context"

	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	buildapp "github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/application/component"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/application/deps"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	gitapp "github.com/liapoldus/liapoldus/backend/internal/application/git"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/runtime"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	"github.com/liapoldus/liapoldus/backend/internal/config"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/artifactstore"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/builder"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/materializer"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/layout"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/registry"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/store"
	gitrepo "github.com/liapoldus/liapoldus/backend/internal/infra/git"
)

// Services bundles all aggregate services wired to one storage and the
// externally-configured defaults.
type Services struct {
	Store      domain.Storage
	Sites      *site.Service
	Pages      *page.Service
	Snapshots  *snapshot.Service
	Contents   *content.Service
	Assets     *asset.Service
	Routes     *route.Service
	Forms      *form.Service
	Components *component.Service
	Builds     *buildapp.Service
	Runtime    *runtime.Service
	Deps       *deps.Service
}

// New builds every aggregate service from the given storage, blob store and
// configuration. Sub-packages never import back into this root package, so no
// import cycle exists.
func New(storage domain.Storage, blobs domain.AssetBlobStore, cfg config.Config) *Services {
	redirectAllowed := make(map[int]bool, len(cfg.RedirectAllowedStatuses))
	for _, status := range cfg.RedirectAllowedStatuses {
		redirectAllowed[status] = true
	}
	gitRepo := gitrepo.NewRepo(cfg.LocalGitDir)
	comps := component.NewService(storage, gitapp.NewService(gitRepo, storage))
	artifacts := artifactstore.New(cfg.BuildDir)
	reg := registry.New(cfg.NPMRegistryURL)
	depsStore := store.New(cfg.DepsDir)
	depsLayout := layout.New(layout.LayoutOptions{
		Packages: storage,
		Store:    depsStore,
		Fetch:    tarballFetcher{client: reg},
	})
	builds := buildapp.NewService(
		storage, storage, storage,
		materializer.New(storage, storage, storage, storage, shared.NewResolver(), depsLayout),
		builder.New(),
		artifacts,
	)
	routes := route.NewService(storage, route.Settings{
		DefaultStatus: cfg.RedirectDefaultStatus,
		Allowed:       redirectAllowed,
	})
	depsSvc := deps.NewService(storage, storage, registryAdapter{client: reg})
	return &Services{
		Store: storage,
		Sites: site.NewService(storage, site.Settings{DefaultLocale: cfg.DefaultLocale}),
		Pages: page.NewService(storage, storage, storage, page.Settings{
			InitialVersion: cfg.PageInitialVersion,
			MaxDepth:       cfg.ComponentMaxDepth,
		}),
		Components: comps,
		Snapshots:  snapshot.NewService(storage, storage, storage, depsSvc),
		Builds:     builds,
		Runtime:    runtime.NewService(storage, storage, storage, routes, builds),
		Contents:   content.NewService(storage),
		Assets: asset.NewService(storage, blobs, storage, asset.Settings{
			MasterVariant: cfg.MasterVariantName,
			FallbackName:  cfg.AssetFallbackName,
			FallbackMime:  cfg.AssetFallbackMime,
			URLTemplate:   cfg.AssetFileURLTemplate,
		}),
		Routes: routes,
		Forms:  form.NewService(storage, storage, form.Settings{EmailPattern: cfg.EmailPattern}),
		Deps:   depsSvc,
	}
}

// registryAdapter bridges the infra HTTP registry client onto the application
// Registry interface, keeping infra independent of application types.
type registryAdapter struct {
	client *registry.Client
}

func (a registryAdapter) Resolve(ctx context.Context, name, spec string) (deps.ResolvedVersion, error) {
	resolved, err := a.client.Resolve(ctx, name, spec)
	if err != nil {
		return deps.ResolvedVersion{}, err
	}
	return deps.ResolvedVersion{
		Name:                 resolved.Name,
		Version:              resolved.Version,
		Integrity:            resolved.Integrity,
		TarballURL:           resolved.TarballURL,
		Dependencies:         resolved.Dependencies,
		PeerDependencies:     resolved.PeerDependencies,
		PeerDependenciesMeta: resolved.PeerDependenciesMeta,
	}, nil
}

// tarballFetcher bridges the infra registry client onto the layout component's
// TarballFetcher, passing the tarball URL + integrity from the immutable
// dep_packages cache (never a re-resolve at build time).
type tarballFetcher struct {
	client *registry.Client
}

func (t tarballFetcher) Tarball(ctx context.Context, name, version, tarballURL, integrity string) ([]byte, error) {
	return t.client.Fetch(ctx, registry.ResolvedVersion{
		Name:       name,
		Version:    version,
		TarballURL: tarballURL,
		Integrity:  integrity,
	})
}
