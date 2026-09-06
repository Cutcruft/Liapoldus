package application

import (
	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	buildapp "github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/application/component"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	gitapp "github.com/liapoldus/liapoldus/backend/internal/application/git"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	"github.com/liapoldus/liapoldus/backend/internal/config"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/artifactstore"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/builder"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/materializer"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
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
	builds := buildapp.NewService(
		storage, storage, storage,
		materializer.New(storage, storage, storage, shared.NewResolver()),
		builder.New(),
		artifacts,
	)
	return &Services{
		Store: storage,
		Sites: site.NewService(storage, site.Settings{DefaultLocale: cfg.DefaultLocale}),
		Pages: page.NewService(storage, storage, storage, page.Settings{
			InitialVersion: cfg.PageInitialVersion,
			MaxDepth:       cfg.ComponentMaxDepth,
		}),
		Components: comps,
		Snapshots:  snapshot.NewService(storage, storage, storage),
		Builds:     builds,
		Contents:   content.NewService(storage),
		Assets: asset.NewService(storage, blobs, storage, asset.Settings{
			MasterVariant: cfg.MasterVariantName,
			FallbackName:  cfg.AssetFallbackName,
			FallbackMime:  cfg.AssetFallbackMime,
			URLTemplate:   cfg.AssetFileURLTemplate,
		}),
		Routes: route.NewService(storage, route.Settings{
			DefaultStatus: cfg.RedirectDefaultStatus,
			Allowed:       redirectAllowed,
		}),
		Forms: form.NewService(storage, storage, form.Settings{EmailPattern: cfg.EmailPattern}),
	}
}
