// Package git defines the Repository port (application boundary) used by
// component versioning (R5/R6/R7 in docs/design/frontend.md; §4/§5 of
// docs/backend/components-test-spec.md). The go-git implementation lives in
// internal/infra/git; this package contains only the interface and the
// application-level Service built on it.
package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// File layout inside a component release commit (R5).
const (
	FileDefinition = "src/definition.tsx"
	FileSchema     = "schema.json"
	FileMetadata   = "metadata.json"
)

// Service is the application-layer gateway over a Repository. It owns
// site-repo init, release commits and rollbacks, and rewrites the component
// registry on rollback (§4 of docs/backend/components-test-spec.md).
type Service struct {
	repo Repository
	defs domain.ComponentDefinitionRepository
}

func NewService(repo Repository, defs domain.ComponentDefinitionRepository) *Service {
	return &Service{repo: repo, defs: defs}
}

// InitRepo ensures a per-site repository exists. Idempotent.
func (s *Service) InitRepo(ctx context.Context, siteID string) error {
	return s.repo.Init(ctx, siteID)
}

// Release commits source, schema and metadata as one deterministic commit for
// definitionID and returns the resulting ComponentVersion.
func (s *Service) Release(ctx context.Context, siteID, definitionID, name, source string, schema, metadata map[string]any) (domain.ComponentVersion, error) {
	files, err := releaseFiles(source, schema, metadata)
	if err != nil {
		return domain.ComponentVersion{}, err
	}
	if err := s.repo.Init(ctx, siteID); err != nil {
		return domain.ComponentVersion{}, err
	}
	message := fmt.Sprintf("component %s: %s", definitionID, name)
	sha, err := s.repo.Commit(ctx, siteID, definitionID, message, files)
	if err != nil {
		return domain.ComponentVersion{}, err
	}
	return domain.ComponentVersion{
		ID:           sha,
		SiteID:       siteID,
		DefinitionID: definitionID,
		Message:      message,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

// Releases lists the versions of one definition in chronological order.
func (s *Service) Releases(ctx context.Context, siteID, definitionID string) ([]domain.ComponentVersion, error) {
	shas, err := s.repo.ListVersions(ctx, siteID, definitionID)
	if err != nil {
		return nil, err
	}
	versions := make([]domain.ComponentVersion, 0, len(shas))
	for _, sha := range shas {
		versions = append(versions, domain.ComponentVersion{
			ID:           sha,
			SiteID:       siteID,
			DefinitionID: definitionID,
		})
	}
	return versions, nil
}

// CheckoutVersion returns the file set of one commit by sha.
func (s *Service) CheckoutVersion(ctx context.Context, siteID, sha string) (map[string][]byte, error) {
	return s.repo.Checkout(ctx, siteID, sha)
}

// Rollback rewinds definitionID to sha: the checked-out file set is committed
// as a new version (history grows, never rewrites) and the registry entry is
// updated to point at the rollback commit. If the checkout fails the registry
// is left untouched.
func (s *Service) Rollback(ctx context.Context, siteID, definitionID, sha string) error {
	files, err := s.repo.Checkout(ctx, siteID, sha)
	if err != nil {
		return err
	}
	current, err := s.defs.Get(ctx, siteID, definitionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("%w: component %q", domain.ErrNotFound, definitionID)
		}
		return err
	}
	rolled := &domain.ComponentDefinition{
		SiteID:    siteID,
		ID:        definitionID,
		Name:      current.Name,
		Kind:      current.Kind,
		Source:    string(files[FileDefinition]),
		CreatedAt: current.CreatedAt,
	}
	if err := json.Unmarshal(files[FileSchema], &rolled.Schema); err != nil {
		return fmt.Errorf("%w: schema in %s is not valid json: %v", domain.ErrInvalidRequest, sha, err)
	}
	if err := json.Unmarshal(files[FileMetadata], &rolled.Metadata); err != nil {
		return fmt.Errorf("%w: metadata in %s is not valid json: %v", domain.ErrInvalidRequest, sha, err)
	}
	newSHA, err := s.repo.Commit(ctx, siteID, definitionID, fmt.Sprintf("rollback %s: %s", definitionID, sha), files)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	rolled.CurrentSHA = newSHA
	rolled.UpdatedAt = now
	return s.defs.Save(ctx, rolled)
}

func releaseFiles(source string, schema, metadata map[string]any) (map[string][]byte, error) {
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal schema: %v", domain.ErrInvalidRequest, err)
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal metadata: %v", domain.ErrInvalidRequest, err)
	}
	return map[string][]byte{
		FileDefinition: []byte(source),
		FileSchema:     schemaJSON,
		FileMetadata:   metadataJSON,
	}, nil
}
