package gitsnapshot

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	itemset "github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// loadState scatters a commit tree back into the database:
//   - site patch
//   - pages / contents / routes / forms upserted, orphans deleted
//   - component definitions upserted, orphans deleted
//   - design tokens upserted (absent file resets to the empty set)
//   - deps-lock.json returned so callers can persist it on a Snapshot.
//
// It returns the loaded page files (for snapshot page refs) and the lock.
func (s *Service) loadState(ctx context.Context, siteID string, files map[string][]byte) (map[string]pageFile, domain.SnapshotLock, error) {
	var site siteFile
	if err := parseJSON(files[sitePath], &site); err != nil {
		return nil, domain.SnapshotLock{}, fmt.Errorf("%w: site.json: %v", domain.ErrInvalidRequest, err)
	}
	currentSite, err := s.sites.GetSite(ctx, siteID)
	if err != nil {
		return nil, domain.SnapshotLock{}, fmt.Errorf("get site: %w", err)
	}
	currentSite.Name = site.Name
	currentSite.Slug = site.Slug
	currentSite.DefaultLocale = site.DefaultLocale
	currentSite.Hosts = site.Hosts
	if err := s.sites.UpdateSite(ctx, currentSite); err != nil {
		return nil, domain.SnapshotLock{}, fmt.Errorf("update site: %w", err)
	}

	pageFiles := map[string]pageFile{}
	existingPages, err := s.pages.ListPagesBySite(ctx, siteID)
	if err != nil {
		return nil, domain.SnapshotLock{}, fmt.Errorf("list pages: %w", err)
	}
	for _, p := range existingPages {
		if _, ok := files["pages/"+p.ID+".json"]; !ok {
			if err := s.pages.DeletePage(ctx, p.ID); err != nil {
				return nil, domain.SnapshotLock{}, fmt.Errorf("delete page %s: %w", p.ID, err)
			}
		}
	}
	for name, raw := range files {
		if !strings.HasPrefix(name, "pages/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		pageID := strings.TrimSuffix(strings.TrimPrefix(name, "pages/"), ".json")
		var pf pageFile
		if err := parseJSON(raw, &pf); err != nil {
			return nil, domain.SnapshotLock{}, fmt.Errorf("%w: %s: %v", domain.ErrInvalidRequest, name, err)
		}
		if err := s.upsertPage(ctx, siteID, pageID, pf); err != nil {
			return nil, domain.SnapshotLock{}, err
		}
		pageFiles[pageID] = pf
	}

	existingContents, err := s.contents.ListContentsBySite(ctx, siteID, "")
	if err != nil {
		return nil, domain.SnapshotLock{}, fmt.Errorf("list contents: %w", err)
	}
	for _, c := range existingContents {
		if _, ok := files["contents/"+c.ID+".json"]; !ok {
			if err := s.contents.DeleteContent(ctx, c.ID); err != nil {
				return nil, domain.SnapshotLock{}, fmt.Errorf("delete content %s: %w", c.ID, err)
			}
		}
	}
	for name, raw := range files {
		if !strings.HasPrefix(name, "contents/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		contentID := strings.TrimSuffix(strings.TrimPrefix(name, "contents/"), ".json")
		var cf contentFile
		if err := parseJSON(raw, &cf); err != nil {
			return nil, domain.SnapshotLock{}, fmt.Errorf("%w: %s: %v", domain.ErrInvalidRequest, name, err)
		}
		if err := s.upsertContent(ctx, siteID, contentID, cf); err != nil {
			return nil, domain.SnapshotLock{}, err
		}
	}

	existingRoutes, err := s.routes.ListRoutesBySite(ctx, siteID)
	if err != nil {
		return nil, domain.SnapshotLock{}, fmt.Errorf("list routes: %w", err)
	}
	for _, r := range existingRoutes {
		if _, ok := files["routes/"+r.ID+".json"]; !ok {
			if err := s.routes.DeleteRoute(ctx, siteID, r.ID); err != nil {
				return nil, domain.SnapshotLock{}, fmt.Errorf("delete route %s: %w", r.ID, err)
			}
		}
	}
	for name, raw := range files {
		if !strings.HasPrefix(name, "routes/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		routeID := strings.TrimSuffix(strings.TrimPrefix(name, "routes/"), ".json")
		var rf routeFile
		if err := parseJSON(raw, &rf); err != nil {
			return nil, domain.SnapshotLock{}, fmt.Errorf("%w: %s: %v", domain.ErrInvalidRequest, name, err)
		}
		if err := s.upsertRoute(ctx, siteID, routeID, rf); err != nil {
			return nil, domain.SnapshotLock{}, err
		}
	}

	existingForms, err := s.forms.ListFormsBySite(ctx, siteID)
	if err != nil {
		return nil, domain.SnapshotLock{}, fmt.Errorf("list forms: %w", err)
	}
	for _, f := range existingForms {
		if _, ok := files["forms/"+f.ID+".json"]; !ok {
			if err := s.forms.DeleteForm(ctx, siteID, f.ID); err != nil {
				return nil, domain.SnapshotLock{}, fmt.Errorf("delete form %s: %w", f.ID, err)
			}
		}
	}
	for name, raw := range files {
		if !strings.HasPrefix(name, "forms/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		formID := strings.TrimSuffix(strings.TrimPrefix(name, "forms/"), ".json")
		var ff formFile
		if err := parseJSON(raw, &ff); err != nil {
			return nil, domain.SnapshotLock{}, fmt.Errorf("%w: %s: %v", domain.ErrInvalidRequest, name, err)
		}
		if err := s.upsertForm(ctx, siteID, formID, ff); err != nil {
			return nil, domain.SnapshotLock{}, err
		}
	}

	if err := s.loadComponents(ctx, siteID, files); err != nil {
		return nil, domain.SnapshotLock{}, err
	}

	tokens := &domain.TokenSet{}
	if raw, ok := files[tokensPath]; ok {
		if err := parseJSON(raw, tokens); err != nil {
			return nil, domain.SnapshotLock{}, fmt.Errorf("%w: tokens.json: %v", domain.ErrInvalidRequest, err)
		}
	}
	if tokens.Colors == nil {
		tokens.Colors = []domain.ColorToken{}
	}
	if err := s.tokens.UpsertTokens(ctx, siteID, tokens); err != nil {
		return nil, domain.SnapshotLock{}, fmt.Errorf("save tokens: %w", err)
	}

	lock := domain.SnapshotLock{}
	if raw, ok := files[depsLockPath]; ok {
		if err := parseJSON(raw, &lock); err != nil {
			return nil, domain.SnapshotLock{}, fmt.Errorf("%w: deps-lock.json: %v", domain.ErrInvalidRequest, err)
		}
	}
	sort.Slice(lock.Deps, func(i, j int) bool { return lock.Deps[i].Name < lock.Deps[j].Name })
	return pageFiles, lock, nil
}

func (s *Service) upsertPage(ctx context.Context, siteID, pageID string, pf pageFile) error {
	if _, err := s.pages.GetPage(ctx, pageID); err == nil {
		current, err := s.pages.GetPage(ctx, pageID)
		if err != nil {
			return err
		}
		versionID, err := itemset.New(itemset.PageVer)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		current.List = pf.List
		if pf.Version > current.Version {
			current.Version = pf.Version
		} else {
			current.Version++
		}
		current.UpdatedAt = now
		version := domain.PageVersion{ID: versionID, PageID: pageID, Number: current.Version, List: pf.List, CreatedAt: now}
		return s.pages.UpdatePage(ctx, current, version)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("get page %s: %w", pageID, err)
	}
	now := time.Now().UTC()
	version := pf.Version
	if version < 1 {
		version = 1
	}
	versionID, err := itemset.New(itemset.PageVer)
	if err != nil {
		return err
	}
	page := domain.Page{ID: pageID, SiteID: siteID, Name: pageID, Slug: pageID, List: pf.List, Version: version, CreatedAt: now, UpdatedAt: pf.UpdatedAt}
	return s.pages.CreatePage(ctx, page, domain.PageVersion{ID: versionID, PageID: pageID, Number: version, List: pf.List, CreatedAt: now})
}

func (s *Service) upsertContent(ctx context.Context, siteID, contentID string, cf contentFile) error {
	if _, err := s.contents.GetContent(ctx, contentID); err == nil {
		current, err := s.contents.GetContent(ctx, contentID)
		if err != nil {
			return err
		}
		current.CollectionID = cf.CollectionID
		current.Fields = cf.Fields
		current.Translations = cf.Translations
		current.UpdatedAt = time.Now().UTC()
		return s.contents.UpdateContent(ctx, current)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("get content %s: %w", contentID, err)
	}
	now := time.Now().UTC()
	content := domain.Content{ID: contentID, SiteID: siteID, CollectionID: cf.CollectionID, Fields: cf.Fields, Translations: cf.Translations, CreatedAt: now, UpdatedAt: now}
	return s.contents.CreateContent(ctx, content)
}

func (s *Service) upsertRoute(ctx context.Context, siteID, routeID string, rf routeFile) error {
	if _, err := s.routes.GetRoute(ctx, siteID, routeID); err == nil {
		current, err := s.routes.GetRoute(ctx, siteID, routeID)
		if err != nil {
			return err
		}
		current.Matcher = rf.Matcher
		current.Priority = rf.Priority
		current.Action = rf.Action
		current.UpdatedAt = time.Now().UTC()
		return s.routes.UpdateRoute(ctx, current)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("get route %s: %w", routeID, err)
	}
	now := time.Now().UTC()
	route := domain.Route{ID: routeID, SiteID: siteID, Matcher: rf.Matcher, Priority: rf.Priority, Action: rf.Action, CreatedAt: now, UpdatedAt: now}
	return s.routes.CreateRoute(ctx, route)
}

func (s *Service) upsertForm(ctx context.Context, siteID, formID string, ff formFile) error {
	if _, err := s.forms.GetForm(ctx, siteID, formID); err == nil {
		current, err := s.forms.GetForm(ctx, siteID, formID)
		if err != nil {
			return err
		}
		current.Name = ff.Name
		current.Definition = ff.Definition
		current.UpdatedAt = time.Now().UTC()
		return s.forms.UpdateForm(ctx, current)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("get form %s: %w", formID, err)
	}
	now := time.Now().UTC()
	form := domain.Form{ID: formID, SiteID: siteID, Name: ff.Name, Definition: ff.Definition, CreatedAt: now, UpdatedAt: now}
	return s.forms.CreateForm(ctx, form)
}

// loadComponents upserts component definitions from components/{id}/*
// (removing definitions that are absent from the tree).
func (s *Service) loadComponents(ctx context.Context, siteID string, files map[string][]byte) error {
	existing, err := s.components.List(ctx, siteID)
	if err != nil {
		return fmt.Errorf("list components: %w", err)
	}
	for _, c := range existing {
		if _, ok := files[path.Join("components", c.ID, "source.tsx")]; !ok {
			if err := s.components.Delete(ctx, siteID, c.ID); err != nil {
				return fmt.Errorf("delete component %s: %w", c.ID, err)
			}
		}
	}
	ids := map[string]bool{}
	for name := range files {
		if !strings.HasPrefix(name, "components/") {
			continue
		}
		rest := strings.TrimPrefix(name, "components/")
		componentID, _, _ := strings.Cut(rest, "/")
		ids[componentID] = true
	}
	for componentID := range ids {
		source, ok := files[path.Join("components", componentID, "source.tsx")]
		if !ok {
			return fmt.Errorf("%w: %s has no source.tsx", domain.ErrInvalidRequest, componentID)
		}
		def, err := s.components.Get(ctx, siteID, componentID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("get component %s: %w", componentID, err)
		}
		if def == nil {
			def = &domain.ComponentDefinition{}
		}
		def.Source = string(source)
		def.SiteID = siteID
		def.ID = componentID
		if raw, ok := files[path.Join("components", componentID, "schema.json")]; ok {
			_ = parseJSON(raw, &def.Schema)
		}
		if raw, ok := files[path.Join("components", componentID, "metadata.json")]; ok {
			_ = parseJSON(raw, &def.Metadata)
		}
		if def.Schema == nil {
			return fmt.Errorf("%w: %s has no schema.json", domain.ErrInvalidRequest, componentID)
		}
		if def.Name == "" {
			def.Name = componentID
		}
		if def.UpdatedAt.IsZero() {
			def.UpdatedAt = time.Now().UTC()
		}
		if def.CreatedAt.IsZero() {
			def.CreatedAt = time.Now().UTC()
		}
		if err := s.components.Save(ctx, def); err != nil {
			return fmt.Errorf("save component %s: %w", componentID, err)
		}
	}
	return nil
}
