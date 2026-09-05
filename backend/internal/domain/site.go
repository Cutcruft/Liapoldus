package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SiteService struct{ repo SiteRepository }

func NewSiteService(repo SiteRepository) *SiteService { return &SiteService{repo: repo} }

func (s *SiteService) Create(ctx context.Context, name, slug, defaultLocale string, hosts []string) (Site, error) {
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if name == "" || slug == "" {
		return Site{}, fmt.Errorf("%w: name and slug are required", ErrInvalidRequest)
	}
	defaultLocale = strings.TrimSpace(defaultLocale)
	if defaultLocale == "" {
		defaultLocale = "ru"
	}
	hosts = normalizeHosts(hosts)
	id, err := NewID("site")
	if err != nil {
		return Site{}, err
	}
	site := Site{ID: id, Name: name, Slug: slug, DefaultLocale: defaultLocale, Hosts: hosts, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateSite(ctx, site); err != nil {
		return Site{}, err
	}
	return site, nil
}

func (s *SiteService) Get(ctx context.Context, id string) (Site, error) {
	return s.repo.GetSite(ctx, id)
}

func (s *SiteService) GetBySlug(ctx context.Context, slug string) (Site, error) {
	return s.repo.GetSiteBySlug(ctx, slug)
}

func (s *SiteService) List(ctx context.Context) ([]Site, error) {
	return s.repo.ListSites(ctx)
}

// Update applies an admin partial update: empty values leave fields unchanged,
// except Hosts which is replaced when non-nil.
func (s *SiteService) Update(ctx context.Context, id string, patch func(Site) Site) (Site, error) {
	current, err := s.repo.GetSite(ctx, id)
	if err != nil {
		return Site{}, err
	}
	updated := patch(current)
	updated.Name = strings.TrimSpace(updated.Name)
	updated.Slug = strings.TrimSpace(updated.Slug)
	if updated.Name == "" || updated.Slug == "" {
		return Site{}, fmt.Errorf("%w: name and slug are required", ErrInvalidRequest)
	}
	updated.DefaultLocale = strings.TrimSpace(updated.DefaultLocale)
	if updated.DefaultLocale == "" {
		updated.DefaultLocale = current.DefaultLocale
	}
	updated.Hosts = normalizeHosts(updated.Hosts)
	if err := s.repo.UpdateSite(ctx, updated); err != nil {
		return Site{}, err
	}
	return updated, nil
}

func (s *SiteService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.GetSite(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteSite(ctx, id)
}

// ResolveByHost returns the site matching the request host (exact host or
// wildcard *.domain); when nothing matches, falls back to the default slug
// site. The context scope is resolved by the caller (client API server).
func (s *SiteService) ResolveByHost(ctx context.Context, host, defaultSlug string) (Site, error) {
	sites, err := s.repo.ListSites(ctx)
	if err != nil {
		return Site{}, err
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h
	}
	for _, site := range sites {
		if hostMatches(site.Hosts, host) {
			return site, nil
		}
	}
	if defaultSlug != "" {
		return s.repo.GetSiteBySlug(ctx, defaultSlug)
	}
	return Site{}, fmt.Errorf("%w: no site for host %q", ErrNotFound, host)
}

func normalizeHosts(hosts []string) []string {
	if hosts == nil {
		return []string{}
	}
	out := make([]string, 0, len(hosts))
	seen := map[string]bool{}
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

func hostMatches(hosts []string, host string) bool {
	for _, h := range hosts {
		if h == host || (strings.HasPrefix(h, "*.") && strings.HasSuffix(host, h[1:])) {
			return true
		}
	}
	return false
}
