// Package registry fetches npm package metadata directly from an npm registry
// over plain HTTP (no node/npm on the host). It reads "abbreviated" packuments
// (Accept: application/vnd.npm.install-v2+json) and resolves a semver range to
// the highest satisfying published version, carrying the dist integrity.
package registry

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
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
// the canonical tarball URL and the manifest's dependencies plus peer
// dependencies to evaluate the peer policy downstream. PeerDependencies keeps
// the declared peer ranges; PeerDependenciesMeta lists only the peers marked
// optional in peerDependenciesMeta (absent entries are required).
type ResolvedVersion struct {
	Name                 string
	Version              string
	Integrity            string
	TarballURL           string
	Dependencies         map[string]string
	PeerDependencies     map[string]string
	PeerDependenciesMeta map[string]bool
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
	Name                 string                 `json:"name"`
	Version              string                 `json:"version"`
	Dependencies         map[string]string      `json:"dependencies"`
	PeerDependencies     map[string]string      `json:"peerDependencies"`
	PeerDependenciesMeta map[string]peerDepMeta `json:"peerDependenciesMeta"`
	Dist                 packumentDist          `json:"dist"`
}

// peerDepMeta carries the peerDependenciesMeta flags that matter to the peer
// policy (spec §5): a peer marked optional does not have to be satisfiable.
type peerDepMeta struct {
	Optional bool `json:"optional"`
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
	optionalPeers := make(map[string]bool, len(meta.PeerDependenciesMeta))
	for peer, peerMeta := range meta.PeerDependenciesMeta {
		if peerMeta.Optional {
			optionalPeers[peer] = true
		}
	}
	return ResolvedVersion{
		Name:                 name,
		Version:              bestVersion,
		Integrity:            meta.Dist.Integrity,
		TarballURL:           meta.Dist.Tarball,
		Dependencies:         meta.Dependencies,
		PeerDependencies:     meta.PeerDependencies,
		PeerDependenciesMeta: optionalPeers,
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

// Fetch downloads the tarball for a resolved package and verifies its
// integrity digest (sha512 base64, per npm's dist.integrity). A mismatch is a
// hard error: the cache must never serve content that failed supply-chain
// validation (spec §9). An empty integrity is accepted (some registries omit
// it), but when present it must match.
func (c *Client) Fetch(ctx context.Context, resolved ResolvedVersion) ([]byte, error) {
	url := resolved.TarballURL
	if url == "" {
		return nil, fmt.Errorf("npm registry %s@%s: no tarball URL", resolved.Name, resolved.Version)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("npm registry tarball %s@%s: %w", resolved.Name, resolved.Version, err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.http.Do(req)
		if err == nil {
			switch {
			case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusRequestTimeout:
				lastErr = fmt.Errorf("npm registry tarball %s@%s: status %d", resolved.Name, resolved.Version, resp.StatusCode)
				resp.Body.Close()
			case resp.StatusCode == http.StatusOK:
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
				resp.Body.Close()
				if readErr != nil {
					return nil, fmt.Errorf("npm registry tarball %s@%s: read: %w", resolved.Name, resolved.Version, readErr)
				}
				if err := verifyIntegrity(resolved, body); err != nil {
					return nil, err
				}
				return body, nil
			default:
				resp.Body.Close()
				lastErr = fmt.Errorf("npm registry tarball %s@%s: unexpected status %d", resolved.Name, resolved.Version, resp.StatusCode)
			}
		} else {
			lastErr = fmt.Errorf("npm registry tarball %s@%s: %w", resolved.Name, resolved.Version, err)
		}
		if attempt < c.maxRetries {
			if !sleep(ctx, c.backoff<<attempt) {
				return nil, lastErr
			}
		}
	}
	return nil, lastErr
}

// verifyIntegrity checks body against a sha512 base64 digest when one is
// present; a non-emtpy integrity that does not match is a hard error.
func verifyIntegrity(resolved ResolvedVersion, body []byte) error {
	integrity := resolved.Integrity
	if integrity == "" {
		return nil
	}
	parts := strings.Fields(integrity)
	for _, part := range parts {
		// npm digest form: "sha512-<base64>".
		if !strings.HasPrefix(part, "sha512-") {
			continue
		}
		expected, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(part, "sha512-"))
		if err != nil {
			continue
		}
		sum := sha512.Sum512(body)
		if string(sum[:]) != string(expected) {
			return fmt.Errorf("npm registry tarball %s@%s: integrity mismatch", resolved.Name, resolved.Version)
		}
		return nil
	}
	// No usable sha512 in the digest string (e.g. sha1-only): nothing to verify.
	return nil
}
