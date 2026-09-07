package component

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/schema"
)

// idRe keeps definition ids safe as identifiers and asset paths. Dots are
// allowed so hierarchies like "layout.main" work.
var idRe = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)

type Service struct {
	defs domain.ComponentDefinitionRepository
	now  func() time.Time
}

func NewService(defs domain.ComponentDefinitionRepository) *Service {
	return &Service{defs: defs, now: time.Now}
}

// Define registers a new component definition and records its first Version.
// All validation happens before any write, so a rejected define leaves the
// registry untouched.
func (s *Service) Define(ctx context.Context, d domain.ComponentDefinition) (domain.ComponentVersion, error) {
	if err := validate(d); err != nil {
		return domain.ComponentVersion{}, err
	}
	if _, err := s.defs.Get(ctx, d.SiteID, d.ID); err == nil {
		return domain.ComponentVersion{}, fmt.Errorf("%w: component %q", domain.ErrAlreadyExists, d.ID)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.ComponentVersion{}, err
	}
	return s.saveNewVersion(ctx, d, "initial version")
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
	return s.saveNewVersion(ctx, d, "updated version")
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

// Delete removes the definition from the registry. History lives in the
// site-wide snapshot git history and is unaffected.
func (s *Service) Delete(ctx context.Context, siteID, id string) error {
	if _, err := s.Get(ctx, siteID, id); err != nil {
		return err
	}
	return s.defs.Delete(ctx, siteID, id)
}

// MarkCommitted records the git sha that holds this component's state after a
// site-wide commit, without bumping the local version (Define/Update already
// assigned the in-memory version id). Idempotent: re-marking the same sha is a
// no-op write.
func (s *Service) MarkCommitted(ctx context.Context, siteID, id, sha string) error {
	d, err := s.Get(ctx, siteID, id)
	if err != nil {
		return err
	}
	d.CurrentSHA = sha
	return s.defs.Save(ctx, d)
}

// saveNewVersion assigns a generated version id, persists the definition and
// returns the version record.
func (s *Service) saveNewVersion(ctx context.Context, d domain.ComponentDefinition, message string) (domain.ComponentVersion, error) {
	versionID, err := id.New(id.ComponentVer)
	if err != nil {
		return domain.ComponentVersion{}, err
	}
	now := s.now()
	d.CurrentSHA = versionID
	d.UpdatedAt = now
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if err := s.defs.Save(ctx, &d); err != nil {
		return domain.ComponentVersion{}, err
	}
	return domain.ComponentVersion{
		ID:           versionID,
		SiteID:       d.SiteID,
		DefinitionID: d.ID,
		Message:      message,
		CreatedAt:    now,
	}, nil
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
