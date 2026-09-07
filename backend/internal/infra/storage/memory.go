package storage

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

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
	defs        map[string]*domain.ComponentDefinition
	builds      map[string]domain.Build
	deps        map[string]domain.Dependency
	pkgCache    map[string]domain.DepPackage
	allowlist   map[string]map[string]struct{}
	cacheConfig map[string]domain.SiteCacheConfig
	tarballAcc  map[string]domain.TarballAccess
	tokens      map[string]*domain.TokenSet
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
		defs:        make(map[string]*domain.ComponentDefinition),
		builds:      make(map[string]domain.Build),
		deps:        make(map[string]domain.Dependency),
		pkgCache:    make(map[string]domain.DepPackage),
		allowlist:   make(map[string]map[string]struct{}),
		cacheConfig: make(map[string]domain.SiteCacheConfig),
		tarballAcc:  make(map[string]domain.TarballAccess),
		tokens:      make(map[string]*domain.TokenSet),
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
	for k, build := range m.builds {
		if build.SiteID == id {
			delete(m.builds, k)
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
	for k, dep := range m.deps {
		if dep.SiteID == id {
			delete(m.deps, k)
		}
	}
	delete(m.tokens, id)
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
	// Snapshot page lists are bookkeeping — the canonical content lives in the
	// site git tree. Deleting a page prunes its stale refs, never blocks.
	for snapshotID, snapshot := range m.snapshots {
		kept := snapshot.Pages[:0]
		for _, page := range snapshot.Pages {
			if page.PageID != id {
				kept = append(kept, page)
			}
		}
		snapshot.Pages = kept
		m.snapshots[snapshotID] = snapshot
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

func (m *Memory) DeleteSubmission(_ context.Context, siteID, formID, submissionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	form, ok := m.forms[formID]
	if !ok || form.SiteID != siteID {
		return domain.ErrNotFound
	}
	list := m.submissions[formID]
	for i, submission := range list {
		if submission.ID != submissionID {
			continue
		}
		m.submissions[formID] = append(list[:i], list[i+1:]...)
		return nil
	}
	return domain.ErrNotFound
}

func (m *Memory) defKey(siteID, id string) string { return siteID + "\x00" + id }

func (m *Memory) depsKey(siteID, name string) string { return siteID + "\x00" + name }

func (m *Memory) Save(_ context.Context, def *domain.ComponentDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defs[m.defKey(def.SiteID, def.ID)] = def
	return nil
}

func (m *Memory) Get(_ context.Context, siteID, id string) (*domain.ComponentDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	def, ok := m.defs[m.defKey(siteID, id)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneDef(def), nil
}

func (m *Memory) List(_ context.Context, siteID string) ([]domain.ComponentDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.ComponentDefinition, 0, len(m.defs))
	for _, def := range m.defs {
		if def.SiteID == siteID {
			result = append(result, *cloneDef(def))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (m *Memory) Delete(_ context.Context, siteID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.defs[m.defKey(siteID, id)]; !ok {
		return domain.ErrNotFound
	}
	delete(m.defs, m.defKey(siteID, id))
	return nil
}

func (m *Memory) CreateBuild(_ context.Context, build domain.Build) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.builds[build.ID] = clone(build)
	return nil
}

func (m *Memory) GetBuild(_ context.Context, id string) (domain.Build, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	build, ok := m.builds[id]
	if !ok {
		return domain.Build{}, domain.ErrNotFound
	}
	return clone(build), nil
}

func (m *Memory) ListBuildsBySite(_ context.Context, siteID string) ([]domain.Build, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Build, 0)
	for _, build := range m.builds {
		if build.SiteID == siteID {
			result = append(result, clone(build))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (m *Memory) GetBuildBySnapshot(_ context.Context, siteID, environment, snapshotID string) (domain.Build, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latest *domain.Build
	for _, build := range m.builds {
		if build.SiteID != siteID || build.Environment != environment || build.SnapshotID != snapshotID {
			continue
		}
		if latest == nil || build.CreatedAt.After(latest.CreatedAt) {
			build := clone(build)
			latest = &build
		}
	}
	if latest == nil {
		return domain.Build{}, domain.ErrNotFound
	}
	return *latest, nil
}

func (m *Memory) UpdateBuild(_ context.Context, build domain.Build) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.builds[build.ID]; !ok {
		return domain.ErrNotFound
	}
	m.builds[build.ID] = clone(build)
	return nil
}

func (m *Memory) CreateDependency(_ context.Context, dep domain.Dependency) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.depsKey(dep.SiteID, dep.Name)
	if _, ok := m.deps[key]; ok {
		return domain.ErrAlreadyExists
	}
	m.deps[key] = clone(dep)
	return nil
}

func (m *Memory) GetDependency(_ context.Context, siteID, name string) (domain.Dependency, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dep, ok := m.deps[m.depsKey(siteID, name)]
	if !ok {
		return domain.Dependency{}, domain.ErrNotFound
	}
	return clone(dep), nil
}

func (m *Memory) ListDependenciesBySite(_ context.Context, siteID string) ([]domain.Dependency, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Dependency, 0)
	for _, dep := range m.deps {
		if dep.SiteID == siteID {
			result = append(result, clone(dep))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (m *Memory) UpdateDependency(_ context.Context, dep domain.Dependency) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.depsKey(dep.SiteID, dep.Name)
	if _, ok := m.deps[key]; !ok {
		return domain.ErrNotFound
	}
	m.deps[key] = clone(dep)
	return nil
}

func (m *Memory) DeleteDependency(_ context.Context, siteID, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.depsKey(siteID, name)
	if _, ok := m.deps[key]; !ok {
		return domain.ErrNotFound
	}
	delete(m.deps, key)
	return nil
}

func (m *Memory) ListAllowlist(_ context.Context, siteID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entries, ok := m.allowlist[siteID]
	if !ok {
		return []string{}, nil
	}
	result := make([]string, 0, len(entries))
	for entry := range entries {
		result = append(result, entry)
	}
	sort.Strings(result)
	return result, nil
}

func (m *Memory) AddAllowlist(_ context.Context, siteID, entry string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries, ok := m.allowlist[siteID]
	if !ok {
		entries = make(map[string]struct{})
		m.allowlist[siteID] = entries
	}
	if _, exists := entries[entry]; exists {
		return domain.ErrAlreadyExists
	}
	entries[entry] = struct{}{}
	return nil
}

func (m *Memory) RemoveAllowlist(_ context.Context, siteID, entry string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries, ok := m.allowlist[siteID]
	if !ok {
		return domain.ErrNotFound
	}
	if _, exists := entries[entry]; !exists {
		return domain.ErrNotFound
	}
	delete(entries, entry)
	if len(entries) == 0 {
		delete(m.allowlist, siteID)
	}
	return nil
}

func (m *Memory) SetCacheConfig(_ context.Context, cfg domain.SiteCacheConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheConfig[cfg.SiteID] = cfg
	return nil
}

func (m *Memory) GetCacheConfig(_ context.Context, siteID string) (domain.SiteCacheConfig, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.cacheConfig[siteID]
	return cfg, ok, nil
}

func (m *Memory) ListCacheConfigs(_ context.Context) ([]domain.SiteCacheConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.SiteCacheConfig, 0, len(m.cacheConfig))
	for _, cfg := range m.cacheConfig {
		result = append(result, cfg)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SiteID < result[j].SiteID })
	return result, nil
}

func (m *Memory) TouchTarballAccess(_ context.Context, name, version string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tarballAcc[name+"\x00"+version] = domain.TarballAccess{Name: name, Version: version, LastAccess: time.Now().UTC()}
	return nil
}

func (m *Memory) ListTarballAccess(_ context.Context) ([]domain.TarballAccess, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.TarballAccess, 0, len(m.tarballAcc))
	for _, acc := range m.tarballAcc {
		result = append(result, acc)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].LastAccess.Equal(result[j].LastAccess) {
			return result[i].Name < result[j].Name
		}
		return result[i].LastAccess.Before(result[j].LastAccess)
	})
	return result, nil
}

func (m *Memory) GetDepPackage(_ context.Context, name, version string) (domain.DepPackage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pkg, ok := m.pkgCache[name+"\x00"+version]
	if !ok {
		return domain.DepPackage{}, domain.ErrNotFound
	}
	return clone(pkg), nil
}

func (m *Memory) CreateDepPackage(_ context.Context, pkg domain.DepPackage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := pkg.Name + "\x00" + pkg.Version
	if _, ok := m.pkgCache[key]; ok {
		return nil
	}
	m.pkgCache[key] = clone(pkg)
	return nil
}

func (m *Memory) GetTokens(_ context.Context, siteID string) (*domain.TokenSet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tokens, ok := m.tokens[siteID]
	if !ok {
		return nil, nil
	}
	return clone(tokens), nil
}

func (m *Memory) UpsertTokens(_ context.Context, siteID string, tokens *domain.TokenSet) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[siteID] = clone(tokens)
	return nil
}

// cloneDef deep-copies a ComponentDefinition. The generic clone() would lose
// Source because it is tagged json:"-" (source lives in git, R5); the registry
// still needs to hand it back to services that reload definitions.
func cloneDef(def *domain.ComponentDefinition) *domain.ComponentDefinition {
	if def == nil {
		return nil
	}
	out := *def
	if def.Schema != nil {
		out.Schema = make(map[string]any, len(def.Schema))
		for k, v := range def.Schema {
			out.Schema[k] = v
		}
	}
	if def.Metadata != nil {
		out.Metadata = make(map[string]any, len(def.Metadata))
		for k, v := range def.Metadata {
			out.Metadata[k] = v
		}
	}
	return &out
}

func clone[T any](value T) T {
	data, _ := json.Marshal(value)
	var result T
	_ = json.Unmarshal(data, &result)
	return result
}
