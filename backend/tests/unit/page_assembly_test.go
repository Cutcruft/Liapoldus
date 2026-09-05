package unit

import (
	"context"
	"errors"
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
		MaxDepth:       8,
		MaxChildren:    100,
	})
}

func node(instanceID string, definitionID string) domain.ComponentNode {
	return domain.ComponentNode{InstanceID: instanceID, DefinitionID: definitionID}
}

func TestPageAssemblyDepthLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	root := node("root", "Container")
	child := node("c1", "Container")
	child.Children = []domain.ComponentNode{node("c2", "Container")}
	child.Children[0].Children = []domain.ComponentNode{node("c3", "Container")}
	child.Children[0].Children[0].Children = []domain.ComponentNode{node("c4", "Container")}
	child.Children[0].Children[0].Children[0].Children = []domain.ComponentNode{node("c5", "Container")}
	child.Children[0].Children[0].Children[0].Children[0].Children = []domain.ComponentNode{node("c6", "Text")}
	child.Children[0].Children[0].Children[0].Children[0].Children[0].Children = []domain.ComponentNode{node("c7", "Text")}
	child.Children[0].Children[0].Children[0].Children[0].Children[0].Children[0].Children = []domain.ComponentNode{node("c8", "Text")}
	child.Children[0].Children[0].Children[0].Children[0].Children[0].Children[0].Children[0].Children = []domain.ComponentNode{node("c9", "Text")}
	child.Children[0].Children[0].Children[0].Children[0].Children[0].Children[0].Children[0].Children[0].Children = []domain.ComponentNode{node("c10", "Text")}
	root.Children = []domain.ComponentNode{child}

	_, err := svc.Create(context.Background(), "site_1", "Deep", "deep", root)
	if err == nil {
		t.Fatalf("expected depth limit error")
	}
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("error %q should mention depth", err)
	}
}

func TestPageAssemblyMissingDefinition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Text"))

	root := node("root", "Missing")
	_, err := svc.Create(context.Background(), "site_1", "Home", "home", root)
	if err == nil {
		t.Fatalf("expected missing definition error")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestPageAssemblyDefinitionOnNestedNode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	nested := node("n1", "Container")
	nested.Children = []domain.ComponentNode{node("n2", "Unknown")}
	root := node("root", "Container")
	root.Children = []domain.ComponentNode{nested}

	_, err := svc.Create(context.Background(), "site_1", "Home", "home", root)
	if err == nil {
		t.Fatalf("expected unknown nested definition error")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestPageAssemblyContentBindingMissingContentID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	text := node("t1", "Text")
	text.Bindings = []domain.ComponentBinding{{
		Property: "text",
		Source:   domain.BindingSource{Type: "content"},
	}}
	root := node("root", "Container")
	root.Children = []domain.ComponentNode{text}

	_, err := svc.Create(context.Background(), "site_1", "Home", "home", root)
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

	text := node("t1", "Text")
	text.Bindings = []domain.ComponentBinding{{
		Property: "text",
		Source:   domain.BindingSource{Type: "magic"},
	}}
	root := node("root", "Container")
	root.Children = []domain.ComponentNode{text}

	_, err := svc.Create(context.Background(), "site_1", "Home", "home", root)
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

	text := node("t1", "Text")
	text.Props = map[string]any{"text": "Default"}
	text.Bindings = []domain.ComponentBinding{{
		Property: "text",
		Source:   domain.BindingSource{Type: "content", ContentID: "col.posts/a1", Path: "title"},
	}}
	root := node("root", "Container")
	root.Children = []domain.ComponentNode{text}

	if _, err := svc.Create(context.Background(), "site_1", "Home", "home", root); err != nil {
		t.Fatalf("valid content binding rejected: %v", err)
	}
}

func TestPageAssemblyPropsAgainstSchema(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	text := node("t1", "Text")
	text.Props = map[string]any{"text": 42}
	root := node("root", "Container")
	root.Children = []domain.ComponentNode{text}

	_, err := svc.Create(context.Background(), "site_1", "Home", "home", root)
	if err == nil {
		t.Fatalf("expected props schema error")
	}
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
	if !strings.Contains(err.Error(), "$.text") {
		t.Fatalf("error %q should contain JSON pointer to path", err)
	}
}

func TestPageAssemblyMissingInstanceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	root := domain.ComponentNode{}
	_, err := svc.Create(context.Background(), "site_1", "Home", "home", root)
	if err == nil {
		t.Fatalf("expected missing instanceId error")
	}
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
	if !strings.Contains(err.Error(), "instanceId") {
		t.Fatalf("error %q should mention instanceId", err)
	}
}

func TestPageAssemblyMissingDefinitionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	root := node("root", "")
	_, err := svc.Create(context.Background(), "site_1", "Home", "home", root)
	if err == nil {
		t.Fatalf("expected missing definitionId error")
	}
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestPageAssemblyMethodNodesInsideText(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().CreatePage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	text := node("t1", "Text")
	text.Children = []domain.ComponentNode{node("btn", "Container")}
	root := node("root", "Container")
	root.Children = []domain.ComponentNode{text}

	if _, err := svc.Create(context.Background(), "site_1", "Home", "home", root); err != nil {
		t.Fatalf("kind constraints are not enforced in this stage: %v", err)
	}
}

func TestPageAssemblyMultipleNodesSameDefinition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().CreatePage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	svc := assemblyService(siteRepo, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	root := node("root", "Container")
	root.Children = []domain.ComponentNode{
		node("t1", "Text"),
		node("t2", "Text"),
		node("t3", "Text"),
	}
	if _, err := svc.Create(context.Background(), "site_1", "Home", "home", root); err != nil {
		t.Fatalf("multiple instances of one definition rejected: %v", err)
	}
}

func TestPageAssemblyUpdateTreeValidatesFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().GetPage(gomock.Any(), "page_1").Return(domain.Page{ID: "page_1", SiteID: "site_1", Version: 1}, nil)
	pageRepo.EXPECT().UpdatePage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	svc := assemblyService(nil, pageRepo, pageDefs(t, "site_1", "Container", "Text"))

	root := node("root", "Container")
	root.Children = []domain.ComponentNode{node("t1", "Text")}
	page, err := svc.UpdateTree(context.Background(), "page_1", root)
	if err != nil {
		t.Fatalf("update tree: %v", err)
	}
	if page.Version != 2 {
		t.Fatalf("version = %d, want 2", page.Version)
	}
}
