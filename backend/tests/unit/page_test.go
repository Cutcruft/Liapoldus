package unit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	pageapp "github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/tests/unit/mocks"
	"go.uber.org/mock/gomock"
)

func newPageService(repo domain.PageRepository, siteRepo domain.SiteRepository) *pageapp.Service {
	return pageapp.NewService(repo, siteRepo, pageapp.Settings{
		InitialVersion: 1,
		MaxDepth:       5,
		Types:          map[string]bool{"Container": true, "Text": true},
	})
}

func validRoot() domain.ComponentNode {
	return domain.ComponentNode{
		ID:       "root",
		Type:     "Container",
		Children: []domain.ComponentNode{{ID: "t1", Type: "Text"}},
	}
}

func TestPageServiceCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().CreatePage(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, page domain.Page, version domain.PageVersion) error {
			if !strings.HasPrefix(page.ID, "page_") {
				t.Fatalf("page id = %q, want page_ prefix", page.ID)
			}
			if page.SiteID != "site_1" || page.Version != 1 {
				t.Fatalf("page = %#v", page)
			}
			if version.Number != 1 || version.PageID != page.ID {
				t.Fatalf("version = %#v", version)
			}
			return nil
		})

	page, err := newPageService(pageRepo, siteRepo).Create(context.Background(), "site_1", "Home", "home", validRoot())
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	if page.Version != 1 || page.SiteID != "site_1" {
		t.Fatalf("page = %#v", page)
	}
}

func TestPageServiceCreateSiteNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_missing").Return(domain.Site{}, domain.ErrNotFound)

	pageRepo := mocks.NewMockPageRepository(ctrl)

	_, err := newPageService(pageRepo, siteRepo).Create(context.Background(), "site_missing", "Home", "home", validRoot())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestPageServiceCreateInvalidRoot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	pageRepo := mocks.NewMockPageRepository(ctrl)
	service := newPageService(pageRepo, siteRepo)

	root := domain.ComponentNode{ID: "root", Type: "Unknown"}
	_, err := service.Create(context.Background(), "site_1", "Home", "home", root)
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestPageServiceUpdateTree(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().GetPage(gomock.Any(), "page_1").Return(domain.Page{ID: "page_1", Version: 1}, nil)
	pageRepo.EXPECT().UpdatePage(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, page domain.Page, version domain.PageVersion) error {
			if page.Version != 2 {
				t.Fatalf("page version = %d, want 2", page.Version)
			}
			if version.Number != 2 {
				t.Fatalf("version number = %d, want 2", version.Number)
			}
			return nil
		})

	page, err := newPageService(pageRepo, nil).UpdateTree(context.Background(), "page_1", validRoot())
	if err != nil {
		t.Fatalf("update tree: %v", err)
	}
	if page.Version != 2 {
		t.Fatalf("page version = %d, want 2", page.Version)
	}
}

func TestPageServiceVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	want := []domain.PageVersion{{ID: "pagever_1", PageID: "page_1", Number: 1}}
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().ListPageVersions(gomock.Any(), "page_1").Return(want, nil)

	versions, err := newPageService(pageRepo, nil).Versions(context.Background(), "page_1")
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 1 || !reflect.DeepEqual(versions[0], want[0]) {
		t.Fatalf("versions = %#v", versions)
	}
}
