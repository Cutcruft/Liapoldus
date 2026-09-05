package domain

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

type RouteActionType string

const (
	RouteRenderPage RouteActionType = "renderPage"
	RouteServeAsset RouteActionType = "serveAsset"
	RouteRedirect   RouteActionType = "redirect"
)

var (
	redirectStatuses = map[int]bool{301: true, 302: true, 307: true, 308: true}
	defaultStatus    = 301
)

// RouteAction mirrors the ui-runtime RouteDescriptor action (docs/ui-runtime/json-descriptors.md §4).
type RouteAction struct {
	Type      RouteActionType `json:"type"`
	PageID    string          `json:"pageId,omitempty"`
	AssetID   string          `json:"assetId,omitempty"`
	Target    string          `json:"target,omitempty"`
	Status    int             `json:"status,omitempty"`
	KeepQuery bool            `json:"keepQuery,omitempty"`
}

type Route struct {
	ID        string      `json:"id"`
	SiteID    string      `json:"siteId"`
	Matcher   string      `json:"matcher"`
	Priority  int         `json:"priority"`
	Action    RouteAction `json:"action"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

type RouteService struct{ repo RouteRepository }

func NewRouteService(repo RouteRepository) *RouteService { return &RouteService{repo: repo} }

func (s *RouteService) Create(ctx context.Context, siteID, matcher string, priority int, action RouteAction) (Route, error) {
	if err := validateRoute("", matcher, priority, action); err != nil {
		return Route{}, err
	}
	id, err := NewID("route")
	if err != nil {
		return Route{}, err
	}
	now := time.Now().UTC()
	r := Route{ID: id, SiteID: siteID, Matcher: matcher, Priority: priority, Action: normalizeAction(action), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateRoute(ctx, r); err != nil {
		return Route{}, err
	}
	return r, nil
}

func (s *RouteService) Get(ctx context.Context, siteID, id string) (Route, error) {
	return s.repo.GetRoute(ctx, siteID, id)
}

func (s *RouteService) List(ctx context.Context, siteID string) ([]Route, error) {
	return s.repo.ListRoutesBySite(ctx, siteID)
}

func (s *RouteService) Update(ctx context.Context, siteID, id string, matcher string, priority int, action RouteAction) (Route, error) {
	current, err := s.repo.GetRoute(ctx, siteID, id)
	if err != nil {
		return Route{}, err
	}
	if matcher == "" {
		matcher = current.Matcher
	}
	if action.Type == "" {
		action = current.Action
	} else {
		action = normalizeAction(action)
	}
	if err := validateRoute(id, matcher, priority, action); err != nil {
		return Route{}, err
	}
	current.Matcher, current.Priority, current.Action, current.UpdatedAt = matcher, priority, action, time.Now().UTC()
	if err := s.repo.UpdateRoute(ctx, current); err != nil {
		return Route{}, err
	}
	return current, nil
}

func (s *RouteService) Delete(ctx context.Context, siteID, id string) error {
	if _, err := s.repo.GetRoute(ctx, siteID, id); err != nil {
		return err
	}
	return s.repo.DeleteRoute(ctx, siteID, id)
}

// Match finds the first route (sorted by priority desc, then creation) matching
// path and returns it with the regexp capture groups for $N expansion.
func (s *RouteService) Match(ctx context.Context, siteID, path string) (Route, []string, bool, error) {
	routes, err := s.repo.ListRoutesBySite(ctx, siteID)
	if err != nil {
		return Route{}, nil, false, err
	}
	sortRoutes(routes)
	for _, r := range routes {
		re, err := regexp.Compile(r.Matcher)
		if err != nil {
			continue
		}
		groups := re.FindStringSubmatch(path)
		if groups == nil {
			continue
		}
		return r, groups[1:], true, nil
	}
	return Route{}, nil, false, nil
}

func normalizeAction(a RouteAction) RouteAction {
	if a.Type == RouteRedirect && a.Status == 0 {
		a.Status = defaultStatus
	}
	return a
}

func validateRoute(id, matcher string, priority int, action RouteAction) error {
	matcher = strings.TrimSpace(matcher)
	if matcher == "" {
		return fmt.Errorf("%w: route matcher is required", ErrInvalidRequest)
	}
	if _, err := regexp.Compile(matcher); err != nil {
		return fmt.Errorf("%w: invalid route matcher: %v", ErrInvalidRequest, err)
	}
	if priority < 0 {
		return fmt.Errorf("%w: route priority must be >= 0", ErrInvalidRequest)
	}
	switch action.Type {
	case RouteRenderPage:
		if strings.TrimSpace(action.PageID) == "" {
			return fmt.Errorf("%w: renderPage requires pageId", ErrInvalidRequest)
		}
	case RouteServeAsset:
		if strings.TrimSpace(action.AssetID) == "" {
			return fmt.Errorf("%w: serveAsset requires assetId", ErrInvalidRequest)
		}
	case RouteRedirect:
		if strings.TrimSpace(action.Target) == "" {
			return fmt.Errorf("%w: redirect requires target", ErrInvalidRequest)
		}
		if action.Status == 0 {
			action.Status = defaultStatus
		}
		if !redirectStatuses[action.Status] {
			return fmt.Errorf("%w: redirect status must be one of 301, 302, 307, 308", ErrInvalidRequest)
		}
	default:
		return fmt.Errorf("%w: unsupported route action %q", ErrInvalidRequest, action.Type)
	}
	return nil
}

func sortRoutes(routes []Route) {
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Priority != routes[j].Priority {
			return routes[i].Priority > routes[j].Priority
		}
		return routes[i].CreatedAt.Before(routes[j].CreatedAt)
	})
}
