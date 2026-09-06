package runtime

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	routeapp "github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Service builds the snapshot-pinned boot contract for a site. The contract
// binds boot to a release: ?versionId selects a snapshot directly, otherwise
// the latest ready build of the environment wins (spec §3: «загрузка привязана
// к версии/снапшоту… Если версия не указана — берётся текущая published
// Билда окружения»).
type Service struct {
	sites     domain.SiteRepository
	snapshots domain.SnapshotRepository
	pages     domain.PageRepository
	routes    *routeapp.Service
	builds    *build.Service
	tokens    domain.TokenRepository
}

func NewService(sites domain.SiteRepository, snapshots domain.SnapshotRepository,
	pages domain.PageRepository, routes *routeapp.Service, builds *build.Service, tokens domain.TokenRepository) *Service {
	return &Service{sites: sites, snapshots: snapshots, pages: pages, routes: routes, builds: builds, tokens: tokens}
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

	tree, err := s.PageTree(ctx, site, environment, versionID, "", "")
	if err != nil {
		return Contract{}, err
	}

	return Contract{
		SiteID:      site.ID,
		Environment: environment,
		Version:     snapshot.ID,
		Locale:      site.DefaultLocale,
		Providers:   []any{},
		Operations:  []any{},
		Endpoints:   []any{},
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
		Tree: tree,
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

// PageTree returns the wire tree of a page in a snapshot release: `pageID`
// selects the page directly (must belong to the snapshot), `routeID` resolves a
// renderPage route to its page, and with neither the boot heuristic applies —
// the renderPage route with the highest priority referencing a snapshot page
// (the home page); otherwise the first page in snapshot order. A snapshot with
// no pages yields nil without error (endpoints return an empty reply, the boot
// contract keeps `tree` omitted).
func (s *Service) PageTree(ctx context.Context, site domain.Site, environment, versionID, pageID, routeID string) (*TreeDeclaration, error) {
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
	return &TreeDeclaration{
		SnapshotID: snapshot.ID,
		VersionID:  version.ID,
		PageID:     selected.PageID,
		Root:       toTreeNode(version.Root),
	}, nil
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

// toTreeNode converts a domain tree into the wire tree that always carries
// props/bindings/children arrays: resolveInstance() in ui-runtime iterates
// bindings/children directly and would crash on undefined.
func toTreeNode(node domain.ComponentNode) *TreeNode {
	return &TreeNode{
		InstanceID:   node.InstanceID,
		DefinitionID: node.DefinitionID,
		Props:        cloneProps(node.Props),
		Bindings:     nonNilBindings(node.Bindings),
		Children:     toTreeNodes(node.Children),
	}
}

// toTreeNodes always returns a non-nil slice so JSON emits `children: []`.
func toTreeNodes(nodes []domain.ComponentNode) []TreeNode {
	out := make([]TreeNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, *toTreeNode(n))
	}
	return out
}

// nonNilBindings guarantees `bindings: []` in JSON instead of null.
func nonNilBindings(bindings []domain.ComponentBinding) []domain.ComponentBinding {
	if bindings == nil {
		return []domain.ComponentBinding{}
	}
	return bindings
}

func cloneProps(p map[string]any) map[string]any {
	if len(p) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
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
