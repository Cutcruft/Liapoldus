// Package registry fetches npm package metadata directly from an npm registry
// over plain HTTP (no node/npm on the host). It reads "abbreviated" packuments
// (Accept: application/vnd.npm.install-v2+json) and resolves a semver range to
// the highest satisfying published version, carrying the dist integrity.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// ResolvedVersion is the outcome of resolving a range against a packument: the
// exact version, its dist integrity (sha512 base64) for supply-chain pinning,
// the canonical tarball URL and the runtime dependencies to resolve next.
type ResolvedVersion struct {
	Name         string
	Version      string
	Integrity    string
	TarballURL   string
	Dependencies map[string]string
}

// Client is a race-free HTTP client for one registry base URL with a small
// in-process packument cache (versions are immutable, so a short TTL suffices
// for repeated transitive resolution).
type Client struct {
	base       string
	http       *http.Client
	cacheTTL   time.Duration
	maxRetries int
	backoff    time.Duration

	mu    sync.Mutex
	packs map[string]*packumentEntry
}

type packumentEntry struct {
	doc     *packument
	fetched time.Time
}

type packument struct {
	Versions map[string]packumentVersion `json:"versions"`
}

type packumentVersion struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
	Dist         packumentDist     `json:"dist"`
}

type packumentDist struct {
	Integrity string `json:"integrity"`
	Tarball   string `json:"tarball"`
}

// New builds a client for baseURL (trailing slash tolerated). Defaults:
// 30s HTTP timeout, 5m packument TTL, 3 attempts with 200ms exponential backoff.
func New(baseURL string) *Client {
	return &Client{
		base:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		http:       &http.Client{Timeout: 30 * time.Second},
		cacheTTL:   5 * time.Minute,
		maxRetries: 2,
		backoff:    200 * time.Millisecond,
		packs:      make(map[string]*packumentEntry),
	}
}

// Resolve picks the highest version satisfying spec from the package's
// abbreviated packument. A missing package yields domain.ErrPackageNotFound;
// a range no published version satisfies yields domain.ErrUnresolvableSpec;
// an unparsable range yields domain.ErrInvalidDepSpec.
func (c *Client) Resolve(ctx context.Context, name, spec string) (ResolvedVersion, error) {
	if !domain.ValidDependencyName(name) {
		return ResolvedVersion{}, fmt.Errorf("%w: %q is not a valid npm package name", domain.ErrInvalidDepSpec, name)
	}
	constraint, err := semver.NewConstraint(strings.TrimSpace(spec))
	if err != nil {
		return ResolvedVersion{}, fmt.Errorf("%w: %q is not a valid npm range (%v)", domain.ErrInvalidDepSpec, spec, err)
	}
	doc, err := c.packument(ctx, name)
	if err != nil {
		return ResolvedVersion{}, err
	}
	var best *semver.Version
	bestVersion := ""
	for version, meta := range doc.Versions {
		if meta.Dist.Tarball == "" {
			continue // unpublished / yanked entries carry no tarball
		}
		v, versionErr := semver.NewVersion(version)
		if versionErr != nil || v == nil {
			continue
		}
		if !constraint.Check(v) {
			continue
		}
		if best == nil || v.GreaterThan(best) {
			best = v
			bestVersion = version
		}
	}
	if best == nil {
		return ResolvedVersion{}, fmt.Errorf("%w: %s@%s", domain.ErrUnresolvableSpec, name, spec)
	}
	meta := doc.Versions[bestVersion]
	return ResolvedVersion{
		Name:         name,
		Version:      bestVersion,
		Integrity:    meta.Dist.Integrity,
		TarballURL:   meta.Dist.Tarball,
		Dependencies: meta.Dependencies,
	}, nil
}

// packument returns the cached abbreviated packument, fetching once on the
// first contact and refreshing only after cacheTTL expires.
func (c *Client) packument(ctx context.Context, name string) (*packument, error) {
	c.mu.Lock()
	if entry, ok := c.packs[name]; ok && time.Since(entry.fetched) < c.cacheTTL {
		c.mu.Unlock()
		return entry.doc, nil
	}
	c.mu.Unlock()

	doc, err := c.fetchPackument(ctx, name)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.packs[name] = &packumentEntry{doc: doc, fetched: time.Now()}
	c.mu.Unlock()
	return doc, nil
}

func (c *Client) fetchPackument(ctx context.Context, name string) (*packument, error) {
	url := c.base + "/" + strings.ReplaceAll(name, "/", "%2F")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("npm registry %s: %w", name, err)
	}
	req.Header.Set("Accept", "application/vnd.npm.install-v2+json; q=1.0, application/json; q=0.8, */*")

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.http.Do(req)
		if err == nil {
			switch {
			case resp.StatusCode == http.StatusNotFound:
				resp.Body.Close()
				return nil, fmt.Errorf("%w: %s", domain.ErrPackageNotFound, name)
			case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusRequestTimeout:
				lastErr = fmt.Errorf("npm registry %s: status %d", name, resp.StatusCode)
				resp.Body.Close()
			case resp.StatusCode == http.StatusOK:
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
				resp.Body.Close()
				if readErr != nil {
					return nil, fmt.Errorf("npm registry %s: read packument: %w", name, readErr)
				}
				var doc packument
				if unmarshalErr := json.Unmarshal(body, &doc); unmarshalErr != nil {
					return nil, fmt.Errorf("npm registry %s: invalid packument: %w", name, unmarshalErr)
				}
				if len(doc.Versions) == 0 {
					return nil, fmt.Errorf("%w: %s (empty packument)", domain.ErrPackageNotFound, name)
				}
				return &doc, nil
			default:
				resp.Body.Close()
				return nil, fmt.Errorf("npm registry %s: unexpected status %d", name, resp.StatusCode)
			}
		} else {
			lastErr = fmt.Errorf("npm registry %s: %w", name, err)
		}
		if attempt < c.maxRetries {
			if !sleep(ctx, c.backoff<<attempt) {
				return nil, lastErr
			}
		}
	}
	return nil, lastErr
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
