package gitsnapshot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Per-component files inside a commit tree (see serialize.go for the layout).
func componentSourcePath(componentID string) string { return "components/" + componentID + "/source.tsx" }
func componentSchemaPath(componentID string) string { return "components/" + componentID + "/schema.json" }
func componentMetadataPath(componentID string) string {
	return "components/" + componentID + "/metadata.json"
}

// DevHeadSHA returns the dev branch HEAD sha, or ErrRepoNotInitialized when
// the site has no repository yet.
func (s *Service) DevHeadSHA(ctx context.Context, siteID string) (string, error) {
	return s.repo.HeadSHA(ctx, siteID, BranchDev)
}

// SourceAt reads a component's source from a commit by sha.
func (s *Service) SourceAt(ctx context.Context, siteID, componentID, sha string) (string, error) {
	data, err := s.repo.ReadFile(ctx, siteID, sha, componentSourcePath(componentID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadComponentAt reconstructs a component definition from a commit tree by
// sha. Only fields stored in git are filled (source, schema, metadata); the
// registry is the authority for name/kind/timestamps.
func (s *Service) ReadComponentAt(ctx context.Context, siteID, componentID, sha string) (*domain.ComponentDefinition, error) {
	files, err := s.repo.ReadFiles(ctx, siteID, sha)
	if err != nil {
		return nil, err
	}
	source, ok := files[componentSourcePath(componentID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	def := &domain.ComponentDefinition{SiteID: siteID, ID: componentID, Source: string(source)}
	if raw, ok := files[componentSchemaPath(componentID)]; ok {
		if err := parseJSON(raw, &def.Schema); err != nil {
			return nil, err
		}
	}
	if raw, ok := files[componentMetadataPath(componentID)]; ok {
		if err := parseJSON(raw, &def.Metadata); err != nil {
			return nil, err
		}
	}
	return def, nil
}

// ComponentCommit is one entry of a component's history: a dev commit where
// the component's source differs from the previous dev commit, with the source
// at that commit inline.
type ComponentCommit struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
	Source  string    `json:"source"`
}

// ComponentHistory walks the dev branch (newest first, at most limit commits)
// and returns the commits where the component's source changed. Consecutive
// commits that did not touch this component are collapsed, so the list maps
// 1:1 to distinct versions of the component.
func (s *Service) ComponentHistory(ctx context.Context, siteID, componentID string, limit int) ([]ComponentCommit, error) {
	commits, err := s.repo.Commits(ctx, siteID, BranchDev, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ComponentCommit, 0, len(commits))
	var prev string
	for _, c := range commits {
		source, err := s.repo.ReadFile(ctx, siteID, c.SHA, componentSourcePath(componentID))
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			return nil, err
		}
		content := string(source)
		if content == prev {
			continue
		}
		prev = content
		out = append(out, ComponentCommit{
			SHA:     c.SHA,
			Message: strings.TrimSpace(c.Message),
			Time:    c.Time,
			Source:  content,
		})
	}
	return out, nil
}