package gitsnapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// File layout committed into git (one commit = one full site state):
//
//	site.json                  — {name, slug, defaultLocale, hosts}
//	pages/{pageID}.json        — {list, version, updatedAt}
//	contents/{contentID}.json  — {collectionId, fields, translations}
//	routes/{routeID}.json      — {matcher, priority, action}
//	forms/{formID}.json        — {name, definition}
//	components/{componentID}/source.tsx
//	components/{componentID}/schema.json
//	components/{componentID}/metadata.json
//	tokens.json               — {colors, typography, spacing, …}
//	deps-lock.json            — {deps: [...]} (frozen dependency graph)
const (
	depsLockPath = "deps-lock.json"
	sitePath     = "site.json"
	tokensPath   = "tokens.json"
)

type siteFile struct {
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	DefaultLocale string   `json:"defaultLocale"`
	Hosts         []string `json:"hosts"`
}

type pageFile struct {
	LayoutSectionID string           `json:"layoutSectionId,omitempty"`
	Head            domain.PageHead  `json:"head,omitempty"`
	List            []domain.Element `json:"list"`
	Version         int32            `json:"version"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

type contentFile struct {
	CollectionID string                    `json:"collectionId"`
	Fields       map[string]any            `json:"fields"`
	Translations map[string]map[string]any `json:"translations"`
}

type routeFile struct {
	Matcher  string             `json:"matcher"`
	Priority int                `json:"priority"`
	Action   domain.RouteAction `json:"action"`
}

type formFile struct {
	Name       string         `json:"name"`
	Definition map[string]any `json:"definition"`
}

// serialize renders the full database state of a site into the git file map.
func (s *Service) serialize(ctx context.Context, siteID string) (map[string][]byte, error) {
	files, err := s.serializeSansDeps(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if s.deps != nil {
		lock, err := s.deps.ResolveLock(ctx, siteID)
		if err != nil {
			return nil, fmt.Errorf("resolve dependencies: %w", err)
		}
		files[depsLockPath] = mustJSON(lock)
	}
	return files, nil
}

// serializeSansDeps renders the state files only (no dependency resolution),
// used for dirty checks and commit contents.
func (s *Service) serializeSansDeps(ctx context.Context, siteID string) (map[string][]byte, error) {
	site, err := s.sites.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	files[sitePath] = mustJSON(siteFile{
		Name:          site.Name,
		Slug:          site.Slug,
		DefaultLocale: site.DefaultLocale,
		Hosts:         site.Hosts,
	})

	pages, err := s.pages.ListPagesBySite(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	for _, p := range pages {
		versions, err := s.pages.ListPageVersions(ctx, p.ID)
		if err != nil {
			return nil, fmt.Errorf("list versions of page %s: %w", p.ID, err)
		}
		if len(versions) == 0 {
			return nil, fmt.Errorf("%w: page %s has no versions", domain.ErrInvalidRequest, p.ID)
		}
		latest := versions[len(versions)-1]
		files["pages/"+p.ID+".json"] = mustJSON(pageFile{
			LayoutSectionID: latest.LayoutSectionID,
			Head:            latest.Head,
			List:            latest.List,
			Version:         latest.Number,
			UpdatedAt:       p.UpdatedAt,
		})
	}

	contents, err := s.contents.ListContentsBySite(ctx, siteID, "")
	if err != nil {
		return nil, fmt.Errorf("list contents: %w", err)
	}
	for _, c := range contents {
		files["contents/"+c.ID+".json"] = mustJSON(contentFile{
			CollectionID: c.CollectionID,
			Fields:       c.Fields,
			Translations: c.Translations,
		})
	}

	routes, err := s.routes.ListRoutesBySite(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	for _, r := range routes {
		files["routes/"+r.ID+".json"] = mustJSON(routeFile{
			Matcher:  r.Matcher,
			Priority: r.Priority,
			Action:   r.Action,
		})
	}

	forms, err := s.forms.ListFormsBySite(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list forms: %w", err)
	}
	for _, f := range forms {
		files["forms/"+f.ID+".json"] = mustJSON(formFile{
			Name:       f.Name,
			Definition: f.Definition,
		})
	}

	components, err := s.components.List(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list components: %w", err)
	}
	for _, c := range components {
		files["components/"+c.ID+"/source.tsx"] = []byte(c.Source)
		files["components/"+c.ID+"/schema.json"] = mustJSON(c.Schema)
		if c.Metadata != nil {
			files["components/"+c.ID+"/metadata.json"] = mustJSON(c.Metadata)
		}
	}

	tokens, err := s.tokens.GetTokens(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("get tokens: %w", err)
	}
	if tokens == nil {
		tokens = &domain.TokenSet{}
	}
	if tokens.Colors == nil {
		tokens.Colors = []domain.ColorToken{}
	}
	files[tokensPath] = mustJSON(tokens)
	return files, nil
}

// mustJSON marshals v as indented JSON. The value set is request-shaped and
// never fails to marshal, so errors panic rather than thread back.
func mustJSON(v any) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("gitsnapshot: marshal %T: %v", v, err))
	}
	return data
}

func parseJSON(data []byte, into any) error {
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("parse %T: %w", into, err)
	}
	return nil
}

// filesEqual compares two flat file maps byte-for-byte.
func filesEqual(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for name, content := range a {
		other, ok := b[name]
		if !ok || !reflect.DeepEqual(content, other) {
			return false
		}
	}
	return true
}

// sortSnapshotPages orders snapshot page refs deterministically.
func sortSnapshotPages(refs []domain.SnapshotPage) {
	sort.Slice(refs, func(i, j int) bool { return refs[i].PageID < refs[j].PageID })
}
