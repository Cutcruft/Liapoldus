package page

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	idgen "github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Defaults applied when Settings leave a limit at its zero value.
const (
	defaultMaxElements = 500
)

// Settings carries the externally-configured structural constraints of a page
// element list.
type Settings struct {
	InitialVersion int32
	MaxElements    int
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

func (s *Service) Create(ctx context.Context, siteID, name, slug string, list []domain.Element) (domain.Page, error) {
	if _, err := s.siteRepo.GetSite(ctx, siteID); err != nil {
		return domain.Page{}, err
	}
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if name == "" || slug == "" {
		return domain.Page{}, fmt.Errorf("%w: name and slug are required", domain.ErrInvalidRequest)
	}
	list, err := s.assignElementIDs(list)
	if err != nil {
		return domain.Page{}, err
	}
	if err := s.validateList(ctx, siteID, list); err != nil {
		return domain.Page{}, err
	}
	id, err := idgen.New(idgen.Page)
	if err != nil {
		return domain.Page{}, err
	}
	now := time.Now().UTC()
	page := domain.Page{ID: id, SiteID: siteID, Name: name, Slug: slug, List: list, Version: s.settings.InitialVersion, CreatedAt: now, UpdatedAt: now}
	versionID, err := idgen.New(idgen.PageVer)
	if err != nil {
		return domain.Page{}, err
	}
	version := domain.PageVersion{ID: versionID, PageID: id, Number: s.settings.InitialVersion, List: list, CreatedAt: now}
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

// Update atomically replaces a page's name and element list (§1.3 backend.md),
// assigning stable ids to any elements that lack one.
func (s *Service) Update(ctx context.Context, id, name string, list []domain.Element) (domain.Page, error) {
	current, err := s.repo.GetPage(ctx, id)
	if err != nil {
		return domain.Page{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Page{}, fmt.Errorf("%w: name is required", domain.ErrInvalidRequest)
	}
	list, err = s.assignElementIDs(list)
	if err != nil {
		return domain.Page{}, err
	}
	if err := s.validateList(ctx, current.SiteID, list); err != nil {
		return domain.Page{}, err
	}
	now := time.Now().UTC()
	current.Name, current.List, current.Version, current.UpdatedAt = name, list, current.Version+1, now
	versionID, err := idgen.New(idgen.PageVer)
	if err != nil {
		return domain.Page{}, err
	}
	version := domain.PageVersion{ID: versionID, PageID: id, Number: current.Version, List: list, CreatedAt: now}
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

// validateList checks a page's element list before writing:
//   - element count within the configured cap
//   - every element has a stable id and a known componentId in the site registry
//   - props follow the §1.3 contract (literal or binding with a valid source)
func (s *Service) validateList(ctx context.Context, siteID string, list []domain.Element) error {
	maxElements := s.settings.MaxElements
	if maxElements == 0 {
		maxElements = defaultMaxElements
	}
	if len(list) > maxElements {
		return fmt.Errorf("%w: page has more than %d elements", domain.ErrInvalidRequest, maxElements)
	}
	seen := map[string]bool{}
	for i, el := range list {
		path := fmt.Sprintf("list[%d]", i)
		if strings.TrimSpace(el.ID) == "" {
			return fmt.Errorf("%w: element id is required at %s", domain.ErrInvalidRequest, path)
		}
		if seen[el.ID] {
			return fmt.Errorf("%w: duplicate element id %q at %s", domain.ErrInvalidRequest, el.ID, path)
		}
		seen[el.ID] = true
		if strings.TrimSpace(el.ComponentID) == "" {
			return fmt.Errorf("%w: componentId is required at %s", domain.ErrInvalidRequest, path)
		}
		def, err := s.defs.Get(ctx, siteID, el.ComponentID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return fmt.Errorf("%w: unknown componentId %q referenced at %s", domain.ErrNotFound, el.ComponentID, path)
			}
			return err
		}
		if err := validateElementProps(el.Props, def.Schema); err != nil {
			return fmt.Errorf("%w: props of %s (componentId %q) invalid: %v", domain.ErrInvalidRequest, path, el.ComponentID, err)
		}
	}
	return nil
}

// validateElementProps validates each declared prop's kind and binding source.
// Literal values are passed through; binding props must carry a valid source.
func validateElementProps(props map[string]domain.ElementProp, schema map[string]any) error {
	for _, p := range props {
		switch p.Kind {
		case "literal":
			// literal value is unconstrained here; deep schema validation is a
			// follow-on, matching the previous tree behaviour of passing props.
		case "binding":
			if p.Source == nil {
				return fmt.Errorf("binding prop requires source")
			}
			if err := validateBindingSource(*p.Source); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported prop kind %q", p.Kind)
		}
	}
	return nil
}

// validateBindingSource enforces the §1.3 contract: a binding points at exactly
// one source and carries that source's discriminants.
func validateBindingSource(b domain.BindingSource) error {
	switch b.Kind {
	case "content":
		if strings.TrimSpace(b.ContentID) == "" || strings.TrimSpace(b.Field) == "" {
			return fmt.Errorf("content source requires contentId and field")
		}
	case "form":
		if strings.TrimSpace(b.FormID) == "" {
			return fmt.Errorf("form source requires formId")
		}
	case "operation":
		if strings.TrimSpace(b.OperationID) == "" {
			return fmt.Errorf("operation source requires operationId")
		}
	case "query":
		if strings.TrimSpace(b.Param) == "" {
			return fmt.Errorf("query source requires param")
		}
	case "routeGroup":
		if b.Index == nil {
			return fmt.Errorf("routeGroup source requires index")
		}
	default:
		return fmt.Errorf("unsupported binding source kind %q", b.Kind)
	}
	return nil
}

// assignElementIDs fills in a stable id for every element that lacks one,
// preserving ids already present (so drag/drop and bindings stay stable).
func (s *Service) assignElementIDs(list []domain.Element) ([]domain.Element, error) {
	out := make([]domain.Element, len(list))
	copy(out, list)
	for i := range out {
		if strings.TrimSpace(out[i].ID) != "" {
			continue
		}
		id, err := idgen.New(idgen.Element)
		if err != nil {
			return nil, err
		}
		out[i].ID = id
	}
	return out, nil
}
