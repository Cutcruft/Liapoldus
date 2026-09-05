package storage

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type Memory struct {
	mu          sync.RWMutex
	sites       map[string]domain.Site
	pages       map[string]domain.Page
	versions    map[string][]domain.PageVersion
	snapshots   map[string]domain.Snapshot
	contents    map[string]domain.Content
	assets      map[string]domain.Asset
	routes      map[string]domain.Route
	forms       map[string]domain.Form
	submissions map[string][]domain.Submission
}

var _ domain.Storage = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{
		sites:       make(map[string]domain.Site),
		pages:       make(map[string]domain.Page),
		versions:    make(map[string][]domain.PageVersion),
		snapshots:   make(map[string]domain.Snapshot),
		contents:    make(map[string]domain.Content),
		assets:      make(map[string]domain.Asset),
		routes:      make(map[string]domain.Route),
		forms:       make(map[string]domain.Form),
		submissions: make(map[string][]domain.Submission),
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

func (m *Memory) ListSites(_ context.Context) ([]domain.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Site, 0, len(m.sites))
	for _, site := range m.sites {
		result = append(result, clone(site))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *Memory) UpdateSite(_ context.Context, site domain.Site) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sites[site.ID]; !ok {
		return domain.ErrNotFound
	}
	m.sites[site.ID] = site
	return nil
}

func (m *Memory) DeleteSite(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sites[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.sites, id)
	for k, page := range m.pages {
		if page.SiteID == id {
			delete(m.pages, k)
			delete(m.versions, k)
		}
	}
	for k, snapshot := range m.snapshots {
		if snapshot.SiteID == id {
			delete(m.snapshots, k)
		}
	}
	for k, content := range m.contents {
		if content.SiteID == id {
			delete(m.contents, k)
		}
	}
	for k, asset := range m.assets {
		if asset.SiteID == id {
			delete(m.assets, k)
		}
	}
	for k, route := range m.routes {
		if route.SiteID == id {
			delete(m.routes, k)
		}
	}
	for k, form := range m.forms {
		if form.SiteID == id {
			delete(m.forms, k)
			delete(m.submissions, k)
		}
	}
	return nil
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

func (m *Memory) GetPageVersion(_ context.Context, pageID, versionID string) (domain.PageVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	versions, ok := m.versions[pageID]
	if !ok {
		return domain.PageVersion{}, domain.ErrNotFound
	}
	for _, version := range versions {
		if version.ID == versionID {
			return clone(version), nil
		}
	}
	return domain.PageVersion{}, domain.ErrNotFound
}

func (m *Memory) DeletePage(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.pages[id]; !ok {
		return domain.ErrNotFound
	}
	for _, snapshot := range m.snapshots {
		for _, page := range snapshot.Pages {
			if page.PageID == id {
				return domain.ErrInvalidRequest
			}
		}
	}
	delete(m.pages, id)
	delete(m.versions, id)
	return nil
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

func (m *Memory) DeleteSnapshot(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.snapshots[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.snapshots, id)
	return nil
}

func (m *Memory) CreateContent(_ context.Context, content domain.Content) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.contents[content.ID]; ok {
		return domain.ErrAlreadyExists
	}
	m.contents[content.ID] = clone(content)
	return nil
}

func (m *Memory) GetContent(_ context.Context, id string) (domain.Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	content, ok := m.contents[id]
	if !ok {
		return domain.Content{}, domain.ErrNotFound
	}
	return clone(content), nil
}

func (m *Memory) ListContentsBySite(_ context.Context, siteID, collectionID string) ([]domain.Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Content, 0)
	for _, content := range m.contents {
		if content.SiteID == siteID && (collectionID == "" || content.CollectionID == collectionID) {
			result = append(result, clone(content))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *Memory) GetContentsByIDs(_ context.Context, siteID string, ids []string) (map[string]domain.Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]domain.Content)
	for _, id := range ids {
		content, ok := m.contents[id]
		if ok && content.SiteID == siteID {
			result[id] = clone(content)
		}
	}
	return result, nil
}

func (m *Memory) UpdateContent(_ context.Context, content domain.Content) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.contents[content.ID]; !ok {
		return domain.ErrNotFound
	}
	m.contents[content.ID] = clone(content)
	return nil
}

func (m *Memory) DeleteContent(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.contents[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.contents, id)
	return nil
}

func (m *Memory) CreateAsset(_ context.Context, asset domain.Asset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.assets[asset.ID]; ok {
		return domain.ErrAlreadyExists
	}
	m.assets[asset.ID] = clone(asset)
	return nil
}

func (m *Memory) GetAsset(_ context.Context, id string) (domain.Asset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	asset, ok := m.assets[id]
	if !ok {
		return domain.Asset{}, domain.ErrNotFound
	}
	return clone(asset), nil
}

func (m *Memory) ListAssetsBySite(_ context.Context, siteID string) ([]domain.Asset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Asset, 0)
	for _, asset := range m.assets {
		if asset.SiteID == siteID {
			result = append(result, clone(asset))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *Memory) DeleteAsset(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.assets[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.assets, id)
	return nil
}

func (m *Memory) CreateRoute(_ context.Context, route domain.Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.routes[route.ID]; ok {
		return domain.ErrAlreadyExists
	}
	m.routes[route.ID] = clone(route)
	return nil
}

func (m *Memory) GetRoute(_ context.Context, siteID, id string) (domain.Route, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	route, ok := m.routes[id]
	if !ok || route.SiteID != siteID {
		return domain.Route{}, domain.ErrNotFound
	}
	return clone(route), nil
}

func (m *Memory) ListRoutesBySite(_ context.Context, siteID string) ([]domain.Route, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Route, 0)
	for _, route := range m.routes {
		if route.SiteID == siteID {
			result = append(result, clone(route))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *Memory) UpdateRoute(_ context.Context, route domain.Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.routes[route.ID]; !ok {
		return domain.ErrNotFound
	}
	m.routes[route.ID] = clone(route)
	return nil
}

func (m *Memory) DeleteRoute(_ context.Context, siteID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	route, ok := m.routes[id]
	if !ok || route.SiteID != siteID {
		return domain.ErrNotFound
	}
	delete(m.routes, id)
	return nil
}

func (m *Memory) CreateForm(_ context.Context, form domain.Form) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.forms[form.ID]; ok {
		return domain.ErrAlreadyExists
	}
	m.forms[form.ID] = clone(form)
	return nil
}

func (m *Memory) GetForm(_ context.Context, siteID, id string) (domain.Form, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	form, ok := m.forms[id]
	if !ok || form.SiteID != siteID {
		return domain.Form{}, domain.ErrNotFound
	}
	return clone(form), nil
}

func (m *Memory) ListFormsBySite(_ context.Context, siteID string) ([]domain.Form, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Form, 0)
	for _, form := range m.forms {
		if form.SiteID == siteID {
			result = append(result, clone(form))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *Memory) UpdateForm(_ context.Context, form domain.Form) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.forms[form.ID]; !ok {
		return domain.ErrNotFound
	}
	m.forms[form.ID] = clone(form)
	return nil
}

func (m *Memory) DeleteForm(_ context.Context, siteID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	form, ok := m.forms[id]
	if !ok || form.SiteID != siteID {
		return domain.ErrNotFound
	}
	delete(m.forms, id)
	delete(m.submissions, id)
	return nil
}

func (m *Memory) CreateSubmission(_ context.Context, submission domain.Submission) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.forms[submission.FormID]; !ok {
		return domain.ErrNotFound
	}
	m.submissions[submission.FormID] = append(m.submissions[submission.FormID], clone(submission))
	return nil
}

func (m *Memory) ListSubmissionsByForm(_ context.Context, siteID, formID string) ([]domain.Submission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	form, ok := m.forms[formID]
	if !ok || form.SiteID != siteID {
		return nil, domain.ErrNotFound
	}
	return clone(m.submissions[formID]), nil
}

func clone[T any](value T) T {
	data, _ := json.Marshal(value)
	var result T
	_ = json.Unmarshal(data, &result)
	return result
}
