package runtime

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	routeapp "github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/sitesettings"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Service builds the snapshot-pinned boot contract for a site. The contract
// binds boot to a release: ?versionId selects a snapshot directly, otherwise
// the latest ready build of the environment wins (spec §3: «загрузка привязана
// к версии/снапшоту… Если версия не указана — берётся текущая published
// Билда окружения»).
type Service struct {
	sites      domain.SiteRepository
	snapshots  domain.SnapshotRepository
	pages      domain.PageRepository
	routes     *routeapp.Service
	builds     *build.Service
	tokens     domain.TokenRepository
	operations domain.OperationRepository
	endpoints  domain.EndpointRepository
	settings   *sitesettings.Service
}

func NewService(sites domain.SiteRepository, snapshots domain.SnapshotRepository,
	pages domain.PageRepository, routes *routeapp.Service, builds *build.Service,
	tokens domain.TokenRepository, operations domain.OperationRepository, endpoints domain.EndpointRepository,
	settings *sitesettings.Service) *Service {
	return &Service{
		sites: sites, snapshots: snapshots, pages: pages, routes: routes, builds: builds, tokens: tokens,
		operations: operations, endpoints: endpoints, settings: settings,
	}
}

var validEnvironments = map[string]bool{
	domain.EnvironmentDevelopment: true,
	domain.EnvironmentProduction:  true,
}

// BootContract resolves the boot release and renders the descriptor contract.
// versionID empty → latest ready build of the environment.
func (s *Service) BootContract(ctx context.Context, site domain.Site, environment, versionID string) (Contract, error) {
	if environment == "" {
		environment = domain.EnvironmentProduction
	}
	if !validEnvironments[environment] {
		return Contract{}, fmt.Errorf("%w: environment must be one of development, production", domain.ErrInvalidRequest)
	}

	snapshot, err := s.resolveSnapshot(ctx, site.ID, environment, versionID)
	if err != nil {
		return Contract{}, err
	}

	routes, err := s.routes.List(ctx, site.ID)
	if err != nil {
		return Contract{}, err
	}

	pages, err := s.PageDescriptors(ctx, site, environment, versionID)
	if err != nil {
		return Contract{}, err
	}

	ops, err := s.operations.ListOperationsBySite(ctx, site.ID)
	if err != nil {
		return Contract{}, err
	}
	eps, err := s.endpoints.ListEndpointsBySite(ctx, site.ID)
	if err != nil {
		return Contract{}, err
	}

	// Site-wide presentation defaults the client merges over per-page
	// overrides (R10 P1, client-side head/layout resolution).
	var head domain.SiteHead
	var defaultLayout string
	if s.settings != nil {
		if settings, err := s.settings.Get(ctx, site.ID); err == nil && settings != nil {
			head = settings.Head
			if head.Meta == nil {
				head.Meta = map[string]string{}
			}
			defaultLayout = settings.DefaultLayoutSectionID
		}
	}

	return Contract{
		SiteID:      site.ID,
		Environment: environment,
		Version:     snapshot.ID,
		Locale:      site.DefaultLocale,
		Providers:   []any{},
		Operations:  toOperationDescriptors(ops),
		Endpoints:   toEndpointDescriptors(eps),
		Routes:      toRouteDescriptors(routes),
		Themes:      []ThemeDescriptor{},
		EnabledChannels: EnabledChannels{
			WS:  true,
			SSE: true,
		},
		Capabilities: Capabilities{
			FormSubmissions: true,
			Dev:             environment == domain.EnvironmentDevelopment,
		},
		Head:                   head,
		DefaultLayoutSectionID: defaultLayout,
		Pages:                  pages,
	}, nil
}

func (s *Service) resolveSnapshot(ctx context.Context, siteID, environment, versionID string) (domain.Snapshot, error) {
	if versionID != "" {
		snapshot, err := s.snapshots.GetSnapshot(ctx, versionID)
		if err != nil {
			return domain.Snapshot{}, err
		}
		if snapshot.SiteID != siteID {
			return domain.Snapshot{}, fmt.Errorf("%w: snapshot does not belong to site", domain.ErrNotFound)
		}
		return snapshot, nil
	}
	build, err := s.builds.Published(ctx, siteID, environment)
	if err != nil {
		return domain.Snapshot{}, err
	}
	return s.snapshots.GetSnapshot(ctx, build.SnapshotID)
}

// PageDescriptors returns the assembled element-list page descriptors for every
// page pinned by the snapshot release (§1.3): pages are separate entities from
// routes; each page carries its flat, order-significant element list.
func (s *Service) PageDescriptors(ctx context.Context, site domain.Site, environment, versionID string) ([]PageDescriptor, error) {
	if environment == "" {
		environment = domain.EnvironmentProduction
	}
	if !validEnvironments[environment] {
		return nil, fmt.Errorf("%w: environment must be one of development, production", domain.ErrInvalidRequest)
	}
	snapshot, err := s.resolveSnapshot(ctx, site.ID, environment, versionID)
	if err != nil {
		return nil, err
	}
	out := make([]PageDescriptor, 0, len(snapshot.Pages))
	for _, p := range snapshot.Pages {
		version, err := s.pages.GetPageVersion(ctx, p.PageID, p.VersionID)
		if err != nil {
			return nil, err
		}
		page, err := s.pages.GetPage(ctx, p.PageID)
		if err != nil {
			return nil, err
		}
		out = append(out, *s.pageDescriptor(p.PageID, page, version))
	}
	return out, nil
}

// Page returns the assembled element-list page descriptor for a page in a
// snapshot release: `pageID` selects the page directly (must belong to the
// snapshot), `routeID` resolves a renderPage route to its page, and with
// neither the boot home-page heuristic applies. A snapshot with no pages yields
// nil without error.
func (s *Service) Page(ctx context.Context, site domain.Site, environment, versionID, pageID, routeID string) (*PageDescriptor, error) {
	if environment == "" {
		environment = domain.EnvironmentProduction
	}
	if !validEnvironments[environment] {
		return nil, fmt.Errorf("%w: environment must be one of development, production", domain.ErrInvalidRequest)
	}
	snapshot, err := s.resolveSnapshot(ctx, site.ID, environment, versionID)
	if err != nil {
		return nil, err
	}
	selected, err := s.selectPage(ctx, site.ID, snapshot, pageID, routeID)
	if err != nil && (pageID == "" && routeID == "") && len(snapshot.Pages) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	version, err := s.pages.GetPageVersion(ctx, selected.PageID, selected.VersionID)
	if err != nil {
		return nil, err
	}
	page, err := s.pages.GetPage(ctx, selected.PageID)
	if err != nil {
		return nil, err
	}
	return s.pageDescriptor(selected.PageID, page, version), nil
}

// pageDescriptor assembles a PageDescriptor from the pinned page version,
// carrying the raw per-page layout override and head (R10 P1). The version is
// the released source of truth for layout/head; page-level fields are the
// fallback so pre-R10 pages (where the version predates the columns) still
// resolve their current values.
func (s *Service) pageDescriptor(pageID string, page domain.Page, version domain.PageVersion) *PageDescriptor {
	d := PageDescriptor{
		ID:       pageID,
		Name:     page.Name,
		Elements: toElementDescriptors(version.List),
	}
	layout := version.LayoutSectionID
	if layout == "" {
		layout = page.LayoutSectionID
	}
	if layout != "" {
		d.LayoutSectionID = layout
	}
	head := version.Head
	if headIsEmpty(head) && !headIsEmpty(page.Head) {
		head = page.Head
	}
	if !headIsEmpty(head) {
		w := PageHead{
			Title:       head.Title,
			Description: head.Description,
			Robots:      head.Robots,
			Canonical:   head.Canonical,
			OG:          head.OG,
			Meta:        head.Meta,
		}
		d.Head = &w
	}
	return &d
}

func headIsEmpty(h domain.PageHead) bool {
	return h.Title == "" && h.Description == "" && h.Robots == "" && h.Canonical == "" &&
		len(h.OG) == 0 && len(h.Meta) == 0
}

func (s *Service) selectPage(ctx context.Context, siteID string, snapshot domain.Snapshot, pageID, routeID string) (domain.SnapshotPage, error) {
	if routeID != "" {
		route, err := s.routes.Get(ctx, siteID, routeID)
		if err != nil {
			return domain.SnapshotPage{}, err
		}
		if route.Action.Type != routeapp.RenderPage || route.Action.PageID == "" {
			return domain.SnapshotPage{}, fmt.Errorf("%w: route is not a renderPage route", domain.ErrInvalidRequest)
		}
		pageID = route.Action.PageID
	}
	if pageID != "" {
		for _, p := range snapshot.Pages {
			if p.PageID == pageID {
				return p, nil
			}
		}
		return domain.SnapshotPage{}, fmt.Errorf("%w: page %q not in snapshot", domain.ErrNotFound, pageID)
	}
	routes, err := allRoutes(ctx, s.routes, siteID)
	if err != nil {
		return domain.SnapshotPage{}, err
	}
	selected, ok := matchHomePage(snapshot.Pages, routes)
	if !ok {
		if len(snapshot.Pages) == 0 {
			return domain.SnapshotPage{}, fmt.Errorf("%w: snapshot has no pages", domain.ErrNotFound)
		}
		selected = snapshot.Pages[0]
	}
	return selected, nil
}

// RouteDescriptors maps the site's routes onto the descriptor form the runtime
// boot contract uses (skipping routes whose matcher is not a valid ^…$ regex).
func (s *Service) RouteDescriptors(ctx context.Context, siteID string) ([]RouteDescriptor, error) {
	routes, err := allRoutes(ctx, s.routes, siteID)
	if err != nil {
		return nil, err
	}
	return toRouteDescriptors(routes), nil
}

// Tokens returns the site's design-token set as a single runtime theme. Only
// the default theme exists today — a non-empty themeID is resolved as the
// default so the editor's `tokens.get` builtin has a stable reply.
func (s *Service) Tokens(ctx context.Context, siteID, themeID string) (ThemeDescriptor, error) {
	tokens, err := s.tokens.GetTokens(ctx, siteID)
	if err != nil {
		return ThemeDescriptor{}, err
	}
	if tokens == nil {
		tokens = &domain.TokenSet{}
	}
	if themeID == "" {
		themeID = "default"
	}
	return ThemeDescriptor{ThemeID: themeID, Tokens: tokenSetMap(tokens)}, nil
}

func allRoutes(ctx context.Context, routes *routeapp.Service, siteID string) ([]domain.Route, error) {
	return routes.List(ctx, siteID)
}

// tokenSetMap flattens the domain TokenSet into the runtime theme token map.
func tokenSetMap(t *domain.TokenSet) map[string]any {
	return map[string]any{
		"colors":      t.Colors,
		"fonts":       t.Fonts,
		"typography":  t.Typography,
		"spacing":     t.Spacing,
		"shadows":     t.Shadows,
		"borders":     t.Borders,
		"breakpoints": t.Breakpoints,
		"zIndex":      t.ZIndex,
		"opacity":     t.Opacity,
		"transitions": t.Transitions,
		"custom":      t.Custom,
	}
}

// toElementDescriptors converts a page's element list into the wire element
// descriptors. props/bindings are always present (non-null) so the ui-runtime
// crawler can iterate them safely.
func toElementDescriptors(list []domain.Element) []ElementDescriptor {
	out := make([]ElementDescriptor, 0, len(list))
	for _, el := range list {
		out = append(out, ElementDescriptor{
			ID:          el.ID,
			ComponentID: el.ComponentID,
			Props:       toPropDescriptors(el.Props),
			Bindings:    nonNilBindings(el.Bindings),
		})
	}
	return out
}

func toPropDescriptors(props map[string]domain.ElementProp) map[string]ElementPropDescriptor {
	if len(props) == 0 {
		return map[string]ElementPropDescriptor{}
	}
	out := make(map[string]ElementPropDescriptor, len(props))
	for k, p := range props {
		out[k] = ElementPropDescriptor{Kind: p.Kind, Value: p.Value, Source: p.Source}
	}
	return out
}

// nonNilBindings guarantees `bindings: []` in JSON instead of null.
func nonNilBindings(bindings []domain.BindingSource) []domain.BindingSource {
	if bindings == nil {
		return []domain.BindingSource{}
	}
	return bindings
}

// matchHomePage returns the snapshot page targeted by the most specific
// renderPage route (priority desc, then creation), if any.
func matchHomePage(pages []domain.SnapshotPage, routes []domain.Route) (domain.SnapshotPage, bool) {
	inSnapshot := make(map[string]bool, len(pages))
	for _, p := range pages {
		inSnapshot[p.PageID] = true
	}
	sorted := append([]domain.Route(nil), routes...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})
	for _, r := range sorted {
		if r.Action.Type == routeapp.RenderPage && inSnapshot[r.Action.PageID] {
			for _, p := range pages {
				if p.PageID == r.Action.PageID {
					return p, true
				}
			}
		}
	}
	return domain.SnapshotPage{}, false
}

// toOperationDescriptors maps stored operation rows onto the ui-runtime
// descriptor form. Invalid rows (empty id/typeOp/method/cache) are skipped, so
// a single bad system row cannot break boot — the site's descriptors still
// load (the client falls back to its builtin list for gaps).
func toOperationDescriptors(ops []domain.Operation) []OperationDescriptor {
	out := make([]OperationDescriptor, 0, len(ops))
	for _, op := range ops {
		if op.ID == "" || op.TypeOp == "" || op.Method == "" || op.Cache == "" {
			continue
		}
		provider := op.Provider
		if provider == "" {
			provider = "liapoldus.builtin"
		}
		out = append(out, OperationDescriptor{
			Kind:       "operation",
			ID:         op.ID,
			TypeOp:     op.TypeOp,
			ProviderID: provider,
			Method:     op.Method,
			Path:       op.Path,
			Params:     op.Params,
			Type:       op.ResultType,
			Cache:      op.Cache,
			TTL:        op.TTL,
			Scope:      op.Scope,
			Poll:       op.Poll,
			Subscribe:  op.Subscribe,
		})
	}
	return out
}

// toEndpointDescriptors maps stored endpoint rows onto the ui-runtime
// descriptor form (skipping rows without an operation id — they cannot resolve
// to a callable operation).
func toEndpointDescriptors(eps []domain.Endpoint) []EndpointDescriptor {
	out := make([]EndpointDescriptor, 0, len(eps))
	for _, ep := range eps {
		if ep.ID == "" || ep.OperationID == "" {
			continue
		}
		out = append(out, EndpointDescriptor{
			Kind:        "endpoint",
			ID:          ep.ID,
			Path:        ep.Path,
			Method:      ep.Method,
			OperationID: ep.OperationID,
		})
	}
	return out
}

// toRouteDescriptors maps domain routes onto ui-runtime RouteDescriptors,
// skipping routes that would fail the runtime's descriptor validation (the
// matcher must be a full regex with ^…$ anchors) so one bad row cannot break
// boot.
func toRouteDescriptors(routes []domain.Route) []RouteDescriptor {
	out := make([]RouteDescriptor, 0, len(routes))
	for _, r := range routes {
		if !isDescriptorRoute(r.Matcher) {
			continue
		}
		out = append(out, RouteDescriptor{
			ID:       r.ID,
			Name:     r.Name,
			Matcher:  r.Matcher,
			Priority: r.Priority,
			Action: RouteAction{
				Type:      string(r.Action.Type),
				PageID:    r.Action.PageID,
				AssetID:   r.Action.AssetID,
				Target:    r.Action.Target,
				Status:    r.Action.Status,
				KeepQuery: r.Action.KeepQuery,
			},
		})
	}
	return out
}

func isDescriptorRoute(matcher string) bool {
	if len(matcher) < 2 || matcher[0] != '^' || matcher[len(matcher)-1] != '$' {
		return false
	}
	_, err := regexp.Compile(matcher)
	return err == nil
}
