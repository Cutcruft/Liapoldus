package page

import (
	"context"
	"fmt"
	"strings"
	"time"

	idgen "github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Settings carries the externally-configured structural constraints of a page
// component tree (previously hardcoded version and validation limits).
type Settings struct {
	InitialVersion int32
	MaxDepth       int
	Types          map[string]bool
}

type Service struct {
	repo     domain.PageRepository
	siteRepo domain.SiteRepository
	settings Settings
}

func NewService(repo domain.PageRepository, siteRepo domain.SiteRepository, settings Settings) *Service {
	return &Service{repo: repo, siteRepo: siteRepo, settings: settings}
}

func (s *Service) Create(ctx context.Context, siteID, name, slug string, root domain.ComponentNode) (domain.Page, error) {
	if _, err := s.siteRepo.GetSite(ctx, siteID); err != nil {
		return domain.Page{}, err
	}
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if name == "" || slug == "" {
		return domain.Page{}, fmt.Errorf("%w: name and slug are required", domain.ErrInvalidRequest)
	}
	if err := validateTree(root, 0, s.settings); err != nil {
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
	if err := validateTree(root, 0, s.settings); err != nil {
		return domain.Page{}, err
	}
	current, err := s.repo.GetPage(ctx, id)
	if err != nil {
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

// validateTree checks the structural invariants required by the first renderer.
// The list of component types is intentionally small until ComponentDefinition
// is introduced as a persisted domain object.
func validateTree(n domain.ComponentNode, depth int, settings Settings) error {
	if depth > settings.MaxDepth {
		return fmt.Errorf("%w: component tree depth exceeds %d", domain.ErrInvalidRequest, settings.MaxDepth)
	}
	if strings.TrimSpace(n.ID) == "" {
		return fmt.Errorf("%w: component id is required", domain.ErrInvalidRequest)
	}
	if !settings.Types[n.Type] {
		return fmt.Errorf("%w: unsupported component type %q", domain.ErrInvalidRequest, n.Type)
	}
	for _, child := range n.Children {
		if err := validateTree(child, depth+1, settings); err != nil {
			return err
		}
	}
	return nil
}
