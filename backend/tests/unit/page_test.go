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

func newPageService(repo domain.PageRepository, siteRepo domain.SiteRepository, defs domain.ComponentDefinitionRepository) *pageapp.Service {
	return pageapp.NewService(repo, siteRepo, defs, pageapp.Settings{
		InitialVersion: 1,
		MaxElements:    500,
	})
}

func pageDefs(t *testing.T, siteID string, ids ...string) domain.ComponentDefinitionRepository {
	t.Helper()
	repo := newFakeDefs()
	for _, id := range ids {
		def := componentDefinition(siteID, id, "Определение "+id)
		switch id {
		case "Text":
			def.Schema = mustJSONMap(`{"type":"object","properties":{"text":{"type":"string"}}}`)
		case "Shell":
			def.Schema = mustJSONMap(`{"type":"object","properties":{"slot":{"type":"string"}}}`)
			def.AcceptsPageContent = true
		default:
			def.Schema = mustJSONMap(`{"type":"object","properties":{"gap":{"type":"number"}}}`)
		}
		def.IsSection = true
		if err := repo.Save(context.Background(), &def); err != nil {
			t.Fatalf("seed definition %s: %v", id, err)
		}
	}
	return repo
}

func validList() []domain.Element {
	return []domain.Element{
		{ID: "root", ComponentID: "Container", Props: map[string]domain.ElementProp{"gap": {Kind: "literal", Value: 8}}},
		{ID: "t1", ComponentID: "Text", Props: map[string]domain.ElementProp{"text": {Kind: "literal", Value: "Hello"}}},
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

	page, err := newPageService(pageRepo, siteRepo, pageDefs(t, "site_1", "Container", "Text")).Create(context.Background(), "site_1", "Home", "home", validList(), "", domain.PageHead{})
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

	_, err := newPageService(pageRepo, siteRepo, pageDefs(t, "site_1", "Container", "Text")).Create(context.Background(), "site_missing", "Home", "home", validList(), "", domain.PageHead{})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestPageServiceCreateInvalidElement(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	pageRepo := mocks.NewMockPageRepository(ctrl)
	service := newPageService(pageRepo, siteRepo, pageDefs(t, "site_1", "Container", "Text"))

	list := []domain.Element{{ID: "root", ComponentID: "Unknown", Props: map[string]domain.ElementProp{}}}
	_, err := service.Create(context.Background(), "site_1", "Home", "home", list, "", domain.PageHead{})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestPageServiceUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().GetPage(gomock.Any(), "page_1").Return(domain.Page{ID: "page_1", SiteID: "site_1", Version: 1}, nil)
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

	defs := pageDefs(t, "site_1", "Container", "Text")
	page, err := newPageService(pageRepo, nil, defs).Update(context.Background(), "page_1", "Home", validList(), "", domain.PageHead{})
	if err != nil {
		t.Fatalf("update: %v", err)
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

	versions, err := newPageService(pageRepo, nil, newFakeDefs()).Versions(context.Background(), "page_1")
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 1 || !reflect.DeepEqual(versions[0], want[0]) {
		t.Fatalf("versions = %#v", versions)
	}
}

func TestPageServiceCreateHeadAndLayout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	head := domain.PageHead{
		Title: "Home", Description: "Сайт", Robots: "index, follow",
		Canonical: "https://demo.test/", OG: map[string]string{"og:type": "website"},
		Meta: map[string]string{"theme-color": "#000"},
	}
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().CreatePage(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, page domain.Page, version domain.PageVersion) error {
			if page.LayoutSectionID != "Shell" {
				t.Fatalf("page.layoutSectionId = %q, want Shell", page.LayoutSectionID)
			}
			if !reflect.DeepEqual(page.Head, head) {
				t.Fatalf("page.head = %#v, want %#v", page.Head, head)
			}
			if version.Number != 1 || version.LayoutSectionID != "Shell" || !reflect.DeepEqual(version.Head, head) {
				t.Fatalf("version head/layout not pinned: %#v", version)
			}
			return nil
		})

	page, err := newPageService(pageRepo, siteRepo, pageDefs(t, "site_1", "Container", "Text", "Shell")).
		Create(context.Background(), "site_1", "Home", "home", validList(), "Shell", head)
	if err != nil {
		t.Fatalf("create page with head/layout: %v", err)
	}
	if page.LayoutSectionID != "Shell" || !reflect.DeepEqual(page.Head, head) {
		t.Fatalf("returned page = %#v", page)
	}
}

func TestPageServiceCreateRejectsInvalidLayout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil).AnyTimes()
	pageRepo := mocks.NewMockPageRepository(ctrl)

	service := newPageService(pageRepo, siteRepo, pageDefs(t, "site_1", "Container", "Text", "Shell"))

	cases := []struct {
		name   string
		layout string
	}{
		{"unknown layout section", "Missing"},
		{"primitive used as layout", "Text"},
		{"section without acceptsPageContent", "Container"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Create(context.Background(), "site_1", "Home", "home", validList(), tc.layout, domain.PageHead{})
			if !errors.Is(err, domain.ErrInvalidRequest) {
				t.Fatalf("error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestPageServiceRejectsInvalidHead(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil).AnyTimes()
	pageRepo := mocks.NewMockPageRepository(ctrl)
	service := newPageService(pageRepo, siteRepo, pageDefs(t, "site_1", "Container", "Text"))

	if _, err := service.Create(context.Background(), "site_1", "Home", "home", validList(), "", domain.PageHead{
		Meta: map[string]string{"": "value"},
	}); !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest for empty meta key", err)
	}
	if _, err := service.Create(context.Background(), "site_1", "Home", "home", validList(), "", domain.PageHead{
		OG: map[string]string{"og:title": "  "},
	}); !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest for empty og value", err)
	}
}

func TestPageServiceUpdatePinsHeadAndLayoutToVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	head := domain.PageHead{Title: "New title", Meta: map[string]string{"x": "y"}}
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().GetPage(gomock.Any(), "page_1").Return(domain.Page{ID: "page_1", SiteID: "site_1", Version: 1}, nil)
	pageRepo.EXPECT().UpdatePage(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, page domain.Page, version domain.PageVersion) error {
			if page.Version != 2 || page.LayoutSectionID != "Shell" || !reflect.DeepEqual(page.Head, head) {
				t.Fatalf("updated page = %#v", page)
			}
			if version.Number != 2 || version.LayoutSectionID != "Shell" || !reflect.DeepEqual(version.Head, head) {
				t.Fatalf("version = %#v, want pinned head/layout", version)
			}
			return nil
		})

	page, err := newPageService(pageRepo, nil, pageDefs(t, "site_1", "Container", "Text", "Shell")).
		Update(context.Background(), "page_1", "Home", validList(), "Shell", head)
	if err != nil {
		t.Fatalf("update with head/layout: %v", err)
	}
	if page.Version != 2 || page.LayoutSectionID != "Shell" {
		t.Fatalf("returned page = %#v", page)
	}
}
