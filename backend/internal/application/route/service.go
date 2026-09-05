package route

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// ActionType aliases the domain type so this package can own the action
// constants without forcing domain -> application imports.
type ActionType = domain.RouteActionType

const (
	RenderPage ActionType = "renderPage"
	ServeAsset ActionType = "serveAsset"
	Redirect   ActionType = "redirect"
)

// Settings carries the externally-configured redirect behavior.
type Settings struct {
	DefaultStatus int // used when a redirect action omits status
	Allowed       map[int]bool
}

type Service struct {
	repo     domain.RouteRepository
	settings Settings
}

func NewService(repo domain.RouteRepository, settings Settings) *Service {
	return &Service{repo: repo, settings: settings}
}

func (s *Service) Create(ctx context.Context, siteID, matcher string, priority int, action domain.RouteAction) (domain.Route, error) {
	if err := s.validateRoute("", matcher, priority, action); err != nil {
		return domain.Route{}, err
	}
	id, err := id.New(id.Route)
	if err != nil {
		return domain.Route{}, err
	}
	now := time.Now().UTC()
	r := domain.Route{ID: id, SiteID: siteID, Matcher: matcher, Priority: priority, Action: s.normalizeAction(action), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateRoute(ctx, r); err != nil {
		return domain.Route{}, err
	}
	return r, nil
}

func (s *Service) Get(ctx context.Context, siteID, id string) (domain.Route, error) {
	return s.repo.GetRoute(ctx, siteID, id)
}

func (s *Service) List(ctx context.Context, siteID string) ([]domain.Route, error) {
	return s.repo.ListRoutesBySite(ctx, siteID)
}

func (s *Service) Update(ctx context.Context, siteID, id string, matcher string, priority int, action domain.RouteAction) (domain.Route, error) {
	current, err := s.repo.GetRoute(ctx, siteID, id)
	if err != nil {
		return domain.Route{}, err
	}
	if matcher == "" {
		matcher = current.Matcher
	}
	if action.Type == "" {
		action = current.Action
	} else {
		action = s.normalizeAction(action)
	}
	if err := s.validateRoute(id, matcher, priority, action); err != nil {
		return domain.Route{}, err
	}
	current.Matcher, current.Priority, current.Action, current.UpdatedAt = matcher, priority, action, time.Now().UTC()
	if err := s.repo.UpdateRoute(ctx, current); err != nil {
		return domain.Route{}, err
	}
	return current, nil
}

func (s *Service) Delete(ctx context.Context, siteID, id string) error {
	if _, err := s.repo.GetRoute(ctx, siteID, id); err != nil {
		return err
	}
	return s.repo.DeleteRoute(ctx, siteID, id)
}

// Match finds the first route (sorted by priority desc, then creation) matching
// path and returns it with the regexp capture groups for $N expansion.
func (s *Service) Match(ctx context.Context, siteID, path string) (domain.Route, []string, bool, error) {
	routes, err := s.repo.ListRoutesBySite(ctx, siteID)
	if err != nil {
		return domain.Route{}, nil, false, err
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
	return domain.Route{}, nil, false, nil
}

func (s *Service) normalizeAction(a domain.RouteAction) domain.RouteAction {
	if a.Type == Redirect && a.Status == 0 {
		a.Status = s.settings.DefaultStatus
	}
	return a
}

func (s *Service) validateRoute(currentID, matcher string, priority int, action domain.RouteAction) error {
	matcher = strings.TrimSpace(matcher)
	if matcher == "" {
		return fmt.Errorf("%w: route matcher is required", domain.ErrInvalidRequest)
	}
	if _, err := regexp.Compile(matcher); err != nil {
		return fmt.Errorf("%w: invalid route matcher: %v", domain.ErrInvalidRequest, err)
	}
	if priority < 0 {
		return fmt.Errorf("%w: route priority must be >= 0", domain.ErrInvalidRequest)
	}
	switch action.Type {
	case RenderPage:
		if strings.TrimSpace(action.PageID) == "" {
			return fmt.Errorf("%w: renderPage requires pageId", domain.ErrInvalidRequest)
		}
	case ServeAsset:
		if strings.TrimSpace(action.AssetID) == "" {
			return fmt.Errorf("%w: serveAsset requires assetId", domain.ErrInvalidRequest)
		}
	case Redirect:
		if strings.TrimSpace(action.Target) == "" {
			return fmt.Errorf("%w: redirect requires target", domain.ErrInvalidRequest)
		}
		if action.Status == 0 {
			action.Status = s.settings.DefaultStatus
		}
		if !s.settings.Allowed[action.Status] {
			return fmt.Errorf("%w: redirect status must be one of %s", domain.ErrInvalidRequest, s.allowedStatuses())
		}
	default:
		return fmt.Errorf("%w: unsupported route action %q", domain.ErrInvalidRequest, action.Type)
	}
	return nil
}

func (s *Service) allowedStatuses() string {
	statuses := make([]string, 0, len(s.settings.Allowed))
	for status := range s.settings.Allowed {
		statuses = append(statuses, fmt.Sprintf("%d", status))
	}
	sort.Strings(statuses)
	return strings.Join(statuses, ", ")
}

func sortRoutes(routes []domain.Route) {
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Priority != routes[j].Priority {
			return routes[i].Priority > routes[j].Priority
		}
		return routes[i].CreatedAt.Before(routes[j].CreatedAt)
	})
}
