package site

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Settings carries the externally-configured defaults this aggregate service
// relies on.
type Settings struct {
	DefaultLocale string
}

type Service struct {
	repo     domain.SiteRepository
	settings Settings
}

func NewService(repo domain.SiteRepository, settings Settings) *Service {
	return &Service{repo: repo, settings: settings}
}

func (s *Service) Create(ctx context.Context, name, slug, defaultLocale string, hosts []string) (domain.Site, error) {
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if name == "" || slug == "" {
		return domain.Site{}, fmt.Errorf("%w: name and slug are required", domain.ErrInvalidRequest)
	}
	defaultLocale = strings.TrimSpace(defaultLocale)
	if defaultLocale == "" {
		defaultLocale = s.settings.DefaultLocale
	}
	hosts = normalizeHosts(hosts)
	id, err := id.New(id.Site)
	if err != nil {
		return domain.Site{}, err
	}
	site := domain.Site{ID: id, Name: name, Slug: slug, DefaultLocale: defaultLocale, Hosts: hosts, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateSite(ctx, site); err != nil {
		return domain.Site{}, err
	}
	return site, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Site, error) {
	return s.repo.GetSite(ctx, id)
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (domain.Site, error) {
	return s.repo.GetSiteBySlug(ctx, slug)
}

func (s *Service) List(ctx context.Context) ([]domain.Site, error) {
	return s.repo.ListSites(ctx)
}

// Update applies an admin partial update: empty values leave fields unchanged,
// except Hosts which is replaced when non-nil.
func (s *Service) Update(ctx context.Context, id string, patch func(domain.Site) domain.Site) (domain.Site, error) {
	current, err := s.repo.GetSite(ctx, id)
	if err != nil {
		return domain.Site{}, err
	}
	updated := patch(current)
	updated.Name = strings.TrimSpace(updated.Name)
	updated.Slug = strings.TrimSpace(updated.Slug)
	if updated.Name == "" || updated.Slug == "" {
		return domain.Site{}, fmt.Errorf("%w: name and slug are required", domain.ErrInvalidRequest)
	}
	updated.DefaultLocale = strings.TrimSpace(updated.DefaultLocale)
	if updated.DefaultLocale == "" {
		updated.DefaultLocale = current.DefaultLocale
	}
	updated.Hosts = normalizeHosts(updated.Hosts)
	if err := s.repo.UpdateSite(ctx, updated); err != nil {
		return domain.Site{}, err
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.GetSite(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteSite(ctx, id)
}

// ResolveByHost returns the site matching the request host (exact host or
// wildcard *.domain); when nothing matches, falls back to the default slug
// site. The context scope is resolved by the caller (client API server).
func (s *Service) ResolveByHost(ctx context.Context, host, defaultSlug string) (domain.Site, error) {
	sites, err := s.repo.ListSites(ctx)
	if err != nil {
		return domain.Site{}, err
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
	return domain.Site{}, fmt.Errorf("%w: no site for host %q", domain.ErrNotFound, host)
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
