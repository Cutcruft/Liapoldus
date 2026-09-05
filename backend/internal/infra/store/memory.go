package store

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type Memory struct {
	mu        sync.RWMutex
	sites     map[string]domain.Site
	pages     map[string]domain.Page
	versions  map[string][]domain.PageVersion
	snapshots map[string]domain.Snapshot
}

func NewMemory() *Memory {
	return &Memory{
		sites: make(map[string]domain.Site), pages: make(map[string]domain.Page),
		versions: make(map[string][]domain.PageVersion), snapshots: make(map[string]domain.Snapshot),
	}
}

func (m *Memory) CreateSite(_ context.Context, site domain.Site) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.sites {
		if existing.Slug == site.Slug {
			return domain.ErrAlreadyExists
		}
	}
	m.sites[site.ID] = site
	return nil
}

func (m *Memory) GetSite(_ context.Context, id string) (domain.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	site, ok := m.sites[id]
	if !ok {
		return domain.Site{}, domain.ErrNotFound
	}
	return site, nil
}

func (m *Memory) GetSiteBySlug(_ context.Context, slug string) (domain.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, site := range m.sites {
		if site.Slug == slug {
			return site, nil
		}
	}
	return domain.Site{}, domain.ErrNotFound
}

func (m *Memory) CreatePage(_ context.Context, page domain.Page, version domain.PageVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.pages {
		if existing.SiteID == page.SiteID && existing.Slug == page.Slug {
			return domain.ErrAlreadyExists
		}
	}
	m.pages[page.ID] = clone(page)
	m.versions[page.ID] = []domain.PageVersion{clone(version)}
	return nil
}

func (m *Memory) GetPage(_ context.Context, id string) (domain.Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	page, ok := m.pages[id]
	if !ok {
		return domain.Page{}, domain.ErrNotFound
	}
	return clone(page), nil
}

func (m *Memory) ListPagesBySite(_ context.Context, siteID string) ([]domain.Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Page, 0)
	for _, page := range m.pages {
		if page.SiteID == siteID {
			result = append(result, clone(page))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *Memory) UpdatePage(_ context.Context, page domain.Page, version domain.PageVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.pages[page.ID]; !ok {
		return domain.ErrNotFound
	}
	m.pages[page.ID] = clone(page)
	m.versions[page.ID] = append(m.versions[page.ID], clone(version))
	return nil
}

func (m *Memory) ListPageVersions(_ context.Context, pageID string) ([]domain.PageVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	versions, ok := m.versions[pageID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return clone(versions), nil
}

func (m *Memory) CreateSnapshot(_ context.Context, snapshot domain.Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots[snapshot.ID] = clone(snapshot)
	return nil
}

func (m *Memory) GetSnapshot(_ context.Context, id string) (domain.Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot, ok := m.snapshots[id]
	if !ok {
		return domain.Snapshot{}, domain.ErrNotFound
	}
	return clone(snapshot), nil
}

func (m *Memory) ListSnapshotsBySite(_ context.Context, siteID string) ([]domain.Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Snapshot, 0)
	for _, snapshot := range m.snapshots {
		if snapshot.SiteID == siteID {
			result = append(result, clone(snapshot))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func clone[T any](value T) T {
	data, _ := json.Marshal(value)
	var result T
	_ = json.Unmarshal(data, &result)
	return result
}
