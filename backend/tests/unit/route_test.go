package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	routeapp "github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/tests/unit/mocks"
	"go.uber.org/mock/gomock"
)

func newRouteService(repo domain.RouteRepository) *routeapp.Service {
	return routeapp.NewService(repo, routeapp.Settings{
		DefaultStatus: 301,
		Allowed:       map[int]bool{301: true, 302: true},
	})
}

func TestRouteServiceCreateAppliesDefaultStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockRouteRepository(ctrl)
	repo.EXPECT().CreateRoute(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r domain.Route) error {
			if r.Action.Status != 301 {
				t.Fatalf("status = %d, want default 301", r.Action.Status)
			}
			return nil
		})

	service := newRouteService(repo)
	created, err := service.Create(context.Background(), "site_1", "^/old$", 0, domain.RouteAction{Type: routeapp.Redirect, Target: "/new"})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	if created.Action.Status != 301 {
		t.Fatalf("status = %d, want 301", created.Action.Status)
	}
}

func TestRouteServiceCreateRejectsInvalidStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockRouteRepository(ctrl)

	_, err := newRouteService(repo).Create(context.Background(), "site_1", "^/old$", 0, domain.RouteAction{Type: routeapp.Redirect, Target: "/new", Status: 200})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestRouteServiceCreateRequiresMatcher(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockRouteRepository(ctrl)

	_, err := newRouteService(repo).Create(context.Background(), "site_1", " ", 0, domain.RouteAction{Type: routeapp.RenderPage, PageID: "page_1"})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestRouteServiceMatchPriorityAndGroups(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	routes := []domain.Route{
		{ID: "route_2", SiteID: "site_1", Matcher: "^/shop/(.+)$", Priority: 1, Action: domain.RouteAction{Type: routeapp.Redirect, Target: "/s/$1", Status: 302}, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "route_1", SiteID: "site_1", Matcher: `^/products/([a-z0-9-]+)$`, Priority: 5, Action: domain.RouteAction{Type: routeapp.Redirect, Target: "/shop/$1", Status: 301}, CreatedAt: now.Add(-time.Hour)},
	}
	repo := mocks.NewMockRouteRepository(ctrl)
	repo.EXPECT().ListRoutesBySite(gomock.Any(), "site_1").Return(routes, nil)

	matched, groups, ok, err := newRouteService(repo).Match(context.Background(), "site_1", "/products/sneakers")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if !ok || matched.ID != "route_1" {
		t.Fatalf("matched = %#v, ok = %v", matched, ok)
	}
	if len(groups) != 1 || groups[0] != "sneakers" {
		t.Fatalf("groups = %#v", groups)
	}
}

func TestRouteServiceMatchNoMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockRouteRepository(ctrl)
	repo.EXPECT().ListRoutesBySite(gomock.Any(), "site_1").Return([]domain.Route{
		{ID: "route_1", SiteID: "site_1", Matcher: "^/old$", Priority: 0, Action: domain.RouteAction{Type: routeapp.Redirect, Target: "/new"}},
	}, nil)

	service := newRouteService(repo)
	_, _, ok, err := service.Match(context.Background(), "site_1", "/unknown")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if ok {
		t.Fatal("match = true, want false")
	}
}
