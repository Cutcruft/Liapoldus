package unit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	pageapp "github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/tests/unit/mocks"
	"go.uber.org/mock/gomock"
)

func assemblyService(siteRepo domain.SiteRepository, pageRepo domain.PageRepository, defs domain.ComponentDefinitionRepository) *pageapp.Service {
	return pageapp.NewService(pageRepo, siteRepo, defs, pageapp.Settings{
		InitialVersion: 1,
		MaxElements:    100,
	})
}

func element(componentID string) domain.Element {
	return domain.Element{ID: "el_" + componentID, ComponentID: componentID, Props: map[string]domain.ElementProp{}}
}

func TestPageAssemblyElementCountLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := pageapp.NewService(pageRepo, siteRepo, pageDefs(t, "site_1", "Text"), pageapp.Settings{
		InitialVersion: 1,
		MaxElements:    3,
	})

	var list []domain.Element
	for i := 0; i < 4; i++ {
		e := element("Text")
		e.ID = fmt.Sprintf("el_%d", i)
		list = append(list, e)
	}

	_, err := svc.Create(context.Background(), "site_1", "Deep", "deep", list)
	if err == nil {
		t.Fatalf("expected element count limit error")
	}
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
	if !strings.Contains(err.Error(), "more than") {
		t.Fatalf("error %q should mention the count cap", err)
	}
}

func TestPageAssemblyMissingDefinition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Text"))

	_, err := svc.Create(context.Background(), "site_1", "Home", "home", []domain.Element{element("Missing")})
	if err == nil {
		t.Fatalf("expected missing definition error")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestPageAssemblyDuplicateElementID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	e1 := element("Container")
	e2 := element("Text")
	e2.ID = e1.ID

	_, err := svc.Create(context.Background(), "site_1", "Home", "home", []domain.Element{e1, e2})
	if err == nil {
		t.Fatalf("expected duplicate id error")
	}
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error %q should mention duplicate", err)
	}
}

func TestPageAssemblyContentBindingMissingContentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	text := element("Text")
	text.Props = map[string]domain.ElementProp{
		"text": {Kind: "binding", Source: &domain.BindingSource{Kind: "content"}},
	}

	_, err := svc.Create(context.Background(), "site_1", "Home", "home", []domain.Element{text})
	if err == nil {
		t.Fatalf("expected binding validation error")
	}
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
	if !strings.Contains(err.Error(), "contentId") {
		t.Fatalf("error %q should mention contentId", err)
	}
}

func TestPageAssemblyUnsupportedBindingSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	text := element("Text")
	text.Props = map[string]domain.ElementProp{
		"text": {Kind: "binding", Source: &domain.BindingSource{Kind: "magic"}},
	}

	_, err := svc.Create(context.Background(), "site_1", "Home", "home", []domain.Element{text})
	if err == nil {
		t.Fatalf("expected unsupported binding source error")
	}
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestPageAssemblyValidContentBinding(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().CreatePage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	text := element("Text")
	text.Props = map[string]domain.ElementProp{
		"text": {Kind: "binding", Source: &domain.BindingSource{Kind: "content", ContentID: "col.posts/a1", Field: "title"}},
	}

	if _, err := svc.Create(context.Background(), "site_1", "Home", "home", []domain.Element{text}); err != nil {
		t.Fatalf("valid content binding rejected: %v", err)
	}
}

func TestPageAssemblyAssignsMissingElementID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().CreatePage(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, page domain.Page, version domain.PageVersion) error {
			if len(page.List) != 2 {
				t.Fatalf("expected 2 elements, got %d", len(page.List))
			}
			for _, el := range page.List {
				if el.ID == "" {
					t.Fatalf("element id was not assigned")
				}
			}
			return nil
		})
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	withoutID := element("Container")
	withoutID.ID = ""
	if _, err := svc.Create(context.Background(), "site_1", "Home", "home", []domain.Element{withoutID, element("Text")}); err != nil {
		t.Fatalf("create with missing element ids rejected: %v", err)
	}
}

func TestPageAssemblyMultipleElementsSameDefinition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().CreatePage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	list := []domain.Element{
		{ID: "t1", ComponentID: "Text", Props: map[string]domain.ElementProp{}},
		{ID: "t2", ComponentID: "Text", Props: map[string]domain.ElementProp{}},
		{ID: "t3", ComponentID: "Text", Props: map[string]domain.ElementProp{}},
	}
	if _, err := svc.Create(context.Background(), "site_1", "Home", "home", list); err != nil {
		t.Fatalf("multiple instances of one definition rejected: %v", err)
	}
}

func TestPageAssemblyUpdateValidatesFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().GetPage(gomock.Any(), "page_1").Return(domain.Page{ID: "page_1", SiteID: "site_1", Version: 1}, nil)
	pageRepo.EXPECT().UpdatePage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	svc := assemblyService(nil, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	page, err := svc.Update(context.Background(), "page_1", "Home", []domain.Element{element("Container"), element("Text")})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if page.Version != 2 {
		t.Fatalf("version = %d, want 2", page.Version)
	}
}
