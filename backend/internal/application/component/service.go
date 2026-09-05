package component

import (
	"context"
	"encoding/json"
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

// file layout inside a component commit (R5).
const (
	FileDefinition = "src/definition.tsx"
	FileSchema     = "schema.json"
	FileMetadata   = "metadata.json"
)

type Service struct {
	defs domain.ComponentDefinitionRepository
	git  git.Repository
	now  func() time.Time
}

func NewService(defs domain.ComponentDefinitionRepository, g git.Repository) *Service {
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

// release writes a new version: commits the definition files, then stores the
// registry entry pointing at the new sha.
func (s *Service) release(ctx context.Context, d domain.ComponentDefinition) (domain.ComponentVersion, error) {
	files, err := filesFor(d)
	if err != nil {
		return domain.ComponentVersion{}, err
	}
	message := fmt.Sprintf("component %s: %s", d.ID, d.Name)
	sha, err := s.git.Commit(ctx, d.SiteID, d.ID, message, files)
	if err != nil {
		return domain.ComponentVersion{}, err
	}
	now := s.now()
	d.CurrentSHA = sha
	d.UpdatedAt = now
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if err := s.defs.Save(ctx, &d); err != nil {
		return domain.ComponentVersion{}, err
	}
	return domain.ComponentVersion{
		ID:           sha,
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

// filesFor renders the files that one component release commits (R5).
func filesFor(d domain.ComponentDefinition) (map[string][]byte, error) {
	schemaJSON, err := json.Marshal(d.Schema)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal schema: %v", domain.ErrInvalidRequest, err)
	}
	metadataJSON, err := json.Marshal(d.Metadata)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal metadata: %v", domain.ErrInvalidRequest, err)
	}
	return map[string][]byte{
		FileDefinition: []byte(d.Source),
		FileSchema:     schemaJSON,
		FileMetadata:   metadataJSON,
	}, nil
}
