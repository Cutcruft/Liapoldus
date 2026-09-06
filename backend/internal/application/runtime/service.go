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
}

func NewService(sites domain.SiteRepository, snapshots domain.SnapshotRepository,
	pages domain.PageRepository, routes *routeapp.Service, builds *build.Service) *Service {
	return &Service{sites: sites, snapshots: snapshots, pages: pages, routes: routes, builds: builds}
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

	tree, err := s.initialTree(ctx, snapshot, routes)
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

// initialTree picks the boot page: the renderPage route with the highest
// priority referencing a snapshot page (the home page); otherwise the first
// page in snapshot order. Returns nil when the snapshot has no pages.
func (s *Service) initialTree(ctx context.Context, snapshot domain.Snapshot, routes []domain.Route) (*TreeDeclaration, error) {
	selected, ok := matchHomePage(snapshot.Pages, routes)
	if !ok {
		if len(snapshot.Pages) == 0 {
			return nil, nil
		}
		selected = snapshot.Pages[0]
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
