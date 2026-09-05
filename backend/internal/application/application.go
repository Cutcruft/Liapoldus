package application

import (
	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	"github.com/liapoldus/liapoldus/backend/internal/config"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Services bundles all aggregate services wired to one storage and the
// externally-configured defaults.
type Services struct {
	Sites     *site.Service
	Pages     *page.Service
	Snapshots *snapshot.Service
	Contents  *content.Service
	Assets    *asset.Service
	Routes    *route.Service
	Forms     *form.Service
}

// New builds every aggregate service from the given storage, blob store and
// configuration. Sub-packages never import back into this root package, so no
// import cycle exists.
func New(storage domain.Storage, blobs domain.AssetBlobStore, cfg config.Config) *Services {
	componentTypes := make(map[string]bool, len(cfg.ComponentTypes))
	for _, name := range cfg.ComponentTypes {
		componentTypes[name] = true
	}
	redirectAllowed := make(map[int]bool, len(cfg.RedirectAllowedStatuses))
	for _, status := range cfg.RedirectAllowedStatuses {
		redirectAllowed[status] = true
	}
	return &Services{
		Sites: site.NewService(storage, site.Settings{DefaultLocale: cfg.DefaultLocale}),
		Pages: page.NewService(storage, storage, page.Settings{
			InitialVersion: cfg.PageInitialVersion,
			MaxDepth:       cfg.ComponentMaxDepth,
			Types:          componentTypes,
		}),
		Snapshots: snapshot.NewService(storage, storage, storage),
		Contents:  content.NewService(storage),
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
