package component

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/git"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/schema"
)

// idRe keeps definition ids safe as file paths inside the site repository.
// Dots are allowed so hierarchies like "layout.main" work.
var idRe = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)

type Service struct {
	defs domain.ComponentDefinitionRepository
	git  *git.Service
	now  func() time.Time
}

func NewService(defs domain.ComponentDefinitionRepository, g *git.Service) *Service {
	return &Service{defs: defs, git: g, now: time.Now}
}

// Define registers a new component definition and records its first Version as
// a git commit. All validation happens before any write, so a rejected define
// leaves neither a commit nor registry entry behind.
func (s *Service) Define(ctx context.Context, d domain.ComponentDefinition) (domain.ComponentVersion, error) {
	if err := validate(d); err != nil {
		return domain.ComponentVersion{}, err
	}
	if _, err := s.defs.Get(ctx, d.SiteID, d.ID); err == nil {
		return domain.ComponentVersion{}, fmt.Errorf("%w: component %q", domain.ErrAlreadyExists, d.ID)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.ComponentVersion{}, err
	}
	return s.release(ctx, d)
}

// Update overwrites name/source/schema/metadata of an existing definition and
// records a new Version. Page trees are validated lazily at assembly time, so
// a stricter schema is accepted here (soft content rules, R4).
func (s *Service) Update(ctx context.Context, d domain.ComponentDefinition) (domain.ComponentVersion, error) {
	if err := validate(d); err != nil {
		return domain.ComponentVersion{}, err
	}
	prior, err := s.defs.Get(ctx, d.SiteID, d.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ComponentVersion{}, fmt.Errorf("%w: component %q", domain.ErrNotFound, d.ID)
		}
		return domain.ComponentVersion{}, err
	}
	if d.Kind == "" {
		d.Kind = prior.Kind
	}
	return s.release(ctx, d)
}

func (s *Service) Get(ctx context.Context, siteID, id string) (*domain.ComponentDefinition, error) {
	d, err := s.defs.Get(ctx, siteID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: component %q", domain.ErrNotFound, id)
		}
		return nil, err
	}
	return d, nil
}

func (s *Service) List(ctx context.Context, siteID string) ([]domain.ComponentDefinition, error) {
	return s.defs.List(ctx, siteID)
}

// Delete removes the definition from the registry but leaves git history
// intact (source of truth is the repo, R5).
func (s *Service) Delete(ctx context.Context, siteID, id string) error {
	if _, err := s.Get(ctx, siteID, id); err != nil {
		return err
	}
	return s.defs.Delete(ctx, siteID, id)
}

// Versions lists the git releases of one definition in chronological order.
func (s *Service) Versions(ctx context.Context, siteID, id string) ([]domain.ComponentVersion, error) {
	return s.git.Releases(ctx, siteID, id)
}

// CheckoutVersion returns the file set of one release commit by sha.
func (s *Service) CheckoutVersion(ctx context.Context, siteID, sha string) (map[string][]byte, error) {
	return s.git.CheckoutVersion(ctx, siteID, sha)
}

// Rollback rewinds a definition to a released sha and rewrites the registry.
func (s *Service) Rollback(ctx context.Context, siteID, id, sha string) error {
	return s.git.Rollback(ctx, siteID, id, sha)
}

// release writes a new version: commits the definition files, then stores the
// registry entry pointing at the new sha.
func (s *Service) release(ctx context.Context, d domain.ComponentDefinition) (domain.ComponentVersion, error) {
	ver, err := s.git.Release(ctx, d.SiteID, d.ID, d.Name, d.Source, d.Schema, d.Metadata)
	if err != nil {
		return domain.ComponentVersion{}, err
	}
	now := s.now()
	d.CurrentSHA = ver.ID
	d.UpdatedAt = now
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if err := s.defs.Save(ctx, &d); err != nil {
		return domain.ComponentVersion{}, err
	}
	ver.CreatedAt = now
	return ver, nil
}

func validate(d domain.ComponentDefinition) error {
	if strings.TrimSpace(d.ID) == "" {
		return fmt.Errorf("%w: component id is required", domain.ErrInvalidRequest)
	}
	if !idRe.MatchString(d.ID) {
		return fmt.Errorf("%w: invalid component id %q", domain.ErrInvalidRequest, d.ID)
	}
	if strings.TrimSpace(d.SiteID) == "" {
		return fmt.Errorf("%w: siteId is required", domain.ErrInvalidRequest)
	}
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("%w: component name is required", domain.ErrInvalidRequest)
	}
	if strings.TrimSpace(d.Source) == "" {
		return fmt.Errorf("%w: component source is required", domain.ErrInvalidRequest)
	}
	if err := schema.ValidateSchema(d.Schema); err != nil {
		return err
	}
	return nil
}
