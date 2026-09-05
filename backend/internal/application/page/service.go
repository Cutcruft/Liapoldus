package page

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	idgen "github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/schema"
)

// Defaults applied when Settings leave a limit at its zero value.
const (
	defaultMaxDepth    = 32
	defaultMaxChildren = 100
)

// Settings carries the externally-configured structural constraints of a page
// component tree.
type Settings struct {
	InitialVersion int32
	MaxDepth       int
	MaxChildren    int
}

type Service struct {
	repo     domain.PageRepository
	siteRepo domain.SiteRepository
	defs     domain.ComponentDefinitionRepository
	settings Settings
}

func NewService(repo domain.PageRepository, siteRepo domain.SiteRepository, defs domain.ComponentDefinitionRepository, settings Settings) *Service {
	return &Service{repo: repo, siteRepo: siteRepo, defs: defs, settings: settings}
}

func (s *Service) Create(ctx context.Context, siteID, name, slug string, root domain.ComponentNode) (domain.Page, error) {
	if _, err := s.siteRepo.GetSite(ctx, siteID); err != nil {
		return domain.Page{}, err
	}
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if name == "" || slug == "" {
		return domain.Page{}, fmt.Errorf("%w: name and slug are required", domain.ErrInvalidRequest)
	}
	if err := s.validateTree(ctx, siteID, "$."+root.InstanceID, root, 0); err != nil {
		return domain.Page{}, err
	}
	id, err := idgen.New(idgen.Page)
	if err != nil {
		return domain.Page{}, err
	}
	now := time.Now().UTC()
	page := domain.Page{ID: id, SiteID: siteID, Name: name, Slug: slug, Root: root, Version: s.settings.InitialVersion, CreatedAt: now, UpdatedAt: now}
	versionID, err := idgen.New(idgen.PageVer)
	if err != nil {
		return domain.Page{}, err
	}
	version := domain.PageVersion{ID: versionID, PageID: id, Number: s.settings.InitialVersion, Root: root, CreatedAt: now}
	if err := s.repo.CreatePage(ctx, page, version); err != nil {
		return domain.Page{}, err
	}
	return page, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Page, error) {
	return s.repo.GetPage(ctx, id)
}

func (s *Service) ListBySite(ctx context.Context, siteID string) ([]domain.Page, error) {
	if _, err := s.siteRepo.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.repo.ListPagesBySite(ctx, siteID)
}

func (s *Service) UpdateTree(ctx context.Context, id string, root domain.ComponentNode) (domain.Page, error) {
	current, err := s.repo.GetPage(ctx, id)
	if err != nil {
		return domain.Page{}, err
	}
	if err := s.validateTree(ctx, current.SiteID, "$."+root.InstanceID, root, 0); err != nil {
		return domain.Page{}, err
	}
	now := time.Now().UTC()
	current.Root, current.Version, current.UpdatedAt = root, current.Version+1, now
	versionID, err := idgen.New(idgen.PageVer)
	if err != nil {
		return domain.Page{}, err
	}
	version := domain.PageVersion{ID: versionID, PageID: id, Number: current.Version, Root: root, CreatedAt: now}
	if err := s.repo.UpdatePage(ctx, current, version); err != nil {
		return domain.Page{}, err
	}
	return current, nil
}

func (s *Service) Versions(ctx context.Context, id string) ([]domain.PageVersion, error) {
	return s.repo.ListPageVersions(ctx, id)
}

func (s *Service) Version(ctx context.Context, pageID, versionID string) (domain.PageVersion, error) {
	return s.repo.GetPageVersion(ctx, pageID, versionID)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.GetPage(ctx, id); err != nil {
		return err
	}
	return s.repo.DeletePage(ctx, id)
}

// validateTree checks a page tree before writing:
//   - structural invariants (ids, depth, children count)
//   - every definitionId resolves in the site's component registry
//   - instance props validate against the definition's JSON Schema
//   - bindings follow the §6 contract of docs/ui-runtime/json-descriptors.md
func (s *Service) validateTree(ctx context.Context, siteID, path string, n domain.ComponentNode, depth int) error {
	maxDepth, maxChildren := s.settings.MaxDepth, s.settings.MaxChildren
	if maxDepth == 0 {
		maxDepth = defaultMaxDepth
	}
	if maxChildren == 0 {
		maxChildren = defaultMaxChildren
	}
	if depth > maxDepth {
		return fmt.Errorf("%w: component tree depth exceeds %d", domain.ErrInvalidRequest, maxDepth)
	}
	if len(n.Children) > maxChildren {
		return fmt.Errorf("%w: node at %s has more than %d children", domain.ErrInvalidRequest, path, maxChildren)
	}
	if strings.TrimSpace(n.InstanceID) == "" {
		return fmt.Errorf("%w: instanceId is required at %s", domain.ErrInvalidRequest, path)
	}
	if strings.TrimSpace(n.DefinitionID) == "" {
		return fmt.Errorf("%w: definitionId is required at %s", domain.ErrInvalidRequest, path)
	}
	def, err := s.defs.Get(ctx, siteID, n.DefinitionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("%w: unknown definitionId %q referenced at %s", domain.ErrNotFound, n.DefinitionID, path)
		}
		return err
	}
	if errs := schema.ValidateProps(n.Props, def.Schema); len(errs) > 0 {
		return fmt.Errorf("%w: props of %s (definitionId %q) invalid: %v", domain.ErrInvalidRequest, path, n.DefinitionID, errs[0])
	}
	for _, b := range n.Bindings {
		if err := validateBinding(b); err != nil {
			return fmt.Errorf("%w: binding of %s: %v", domain.ErrInvalidRequest, path, err)
		}
	}
	for _, child := range n.Children {
		if err := s.validateTree(ctx, siteID, path+"."+child.InstanceID, child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// validateBinding enforces the §6 contract: a binding points at exactly one
// source and carries that source's discriminants.
func validateBinding(b domain.ComponentBinding) error {
	if strings.TrimSpace(b.Property) == "" {
		return fmt.Errorf("property is required")
	}
	switch b.Source.Type {
	case "content":
		if strings.TrimSpace(b.Source.ContentID) == "" {
			return fmt.Errorf("content source requires contentId")
		}
	case "routeParam", "routeQuery":
		if strings.TrimSpace(b.Source.Name) == "" {
			return fmt.Errorf("%s source requires name", b.Source.Type)
		}
	case "operation":
		if strings.TrimSpace(b.Source.OperationID) == "" {
			return fmt.Errorf("operation source requires operationId")
		}
	case "form":
		if strings.TrimSpace(b.Source.FormID) == "" {
			return fmt.Errorf("form source requires formId")
		}
	case "props":
		if strings.TrimSpace(b.Source.Path) == "" {
			return fmt.Errorf("props source requires path")
		}
	case "runtime":
		if strings.TrimSpace(b.Source.Source) == "" {
			return fmt.Errorf("runtime source requires source")
		}
	default:
		return fmt.Errorf("unsupported binding source type %q", b.Source.Type)
	}
	return nil
}
