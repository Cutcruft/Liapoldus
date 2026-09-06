package unit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/registry"
)

type packumentVersionFixture struct {
	Name         string               `json:"name"`
	Version      string               `json:"version"`
	Dependencies map[string]string    `json:"dependencies,omitempty"`
	Dist         packumentDistFixture `json:"dist"`
}

type packumentDistFixture struct {
	Integrity string `json:"integrity"`
	Tarball   string `json:"tarball"`
}

func packumentVersion(name, version, depName, depSpec string) packumentVersionFixture {
	v := packumentVersionFixture{
		Name:    name,
		Version: version,
		Dist:    packumentDistFixture{Integrity: "sha512-" + version, Tarball: "https://r.example/" + name + "-" + version + ".tgz"},
	}
	if depName != "" {
		v.Dependencies = map[string]string{depName: depSpec}
	}
	return v
}

func packumentBytes(packs map[string]map[string]packumentVersionFixture) map[string][]byte {
	out := map[string][]byte{}
	for name, versions := range packs {
		doc := struct {
			Versions map[string]packumentVersionFixture `json:"versions"`
		}{Versions: versions}
		raw, err := json.Marshal(doc)
		if err != nil {
			panic(err)
		}
		out[name] = raw
	}
	return out
}

// npmServer serves deterministic abbreviated packuments and records every
// request path plus the Accept header.
type npmServer struct {
	server *httptest.Server
	mu     sync.Mutex
	paths  []string
	accept string
}

func fakeNPM(t *testing.T, packs map[string][]byte) *npmServer {
	t.Helper()
	s := &npmServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.paths = append(s.paths, r.URL.EscapedPath())
		s.accept = r.Header.Get("Accept")
		s.mu.Unlock()

		name := strings.TrimPrefix(r.URL.Path, "/")
		raw, ok := packs[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *npmServer) requestPaths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]string(nil), s.paths...)
	return out
}

func (s *npmServer) acceptHeader() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accept
}

func TestRegistryResolvePicksHighestSatisfying(t *testing.T) {
	packages := packumentBytes(map[string]map[string]packumentVersionFixture{
		"lodash": {
			"1.0.0":        packumentVersion("lodash", "1.0.0", "", ""),
			"1.2.0":        packumentVersion("lodash", "1.2.0", "", ""),
			"2.0.0-beta.1": packumentVersion("lodash", "2.0.0-beta.1", "", ""), // prerelease must not match ^1
			"4.17.21":      packumentVersion("lodash", "4.17.21", "", ""),      // outside ^1 range
		},
	})
	server := fakeNPM(t, packages)
	client := registry.New(server.server.URL)

	resolved, err := client.Resolve(context.Background(), "lodash", "^1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Version != "1.2.0" {
		t.Errorf("Version = %q, want 1.2.0", resolved.Version)
	}
	if resolved.Integrity != "sha512-1.2.0" {
		t.Errorf("Integrity = %q, want sha512-1.2.0", resolved.Integrity)
	}
	if resolved.TarballURL != "https://r.example/lodash-1.2.0.tgz" {
		t.Errorf("TarballURL = %q", resolved.TarballURL)
	}
}

func TestRegistryResolveParsesDependencies(t *testing.T) {
	packages := packumentBytes(map[string]map[string]packumentVersionFixture{
		"agent": {"1.5.0": packumentVersion("agent", "1.5.0", "lodash", "^4")},
	})
	server := fakeNPM(t, packages)
	client := registry.New(server.server.URL)

	resolved, err := client.Resolve(context.Background(), "agent", "^1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := resolved.Dependencies["lodash"]; got != "^4" {
		t.Errorf("Dependencies[lodash] = %q, want ^4", got)
	}
}

func TestRegistryResolveORSpec(t *testing.T) {
	packages := packumentBytes(map[string]map[string]packumentVersionFixture{
		"multi": {
			"1.9.0": packumentVersion("multi", "1.9.0", "", ""),
			"2.0.0": packumentVersion("multi", "2.0.0", "", ""),
			"3.8.0": packumentVersion("multi", "3.8.0", "", ""),
		},
	})
	server := fakeNPM(t, packages)
	client := registry.New(server.server.URL)

	resolved, err := client.Resolve(context.Background(), "multi", "^1 || ^3")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Version != "3.8.0" {
		t.Errorf("Version = %q, want 3.8.0", resolved.Version)
	}
}

func TestRegistryScopedPackageUsesEscapedPath(t *testing.T) {
	packages := packumentBytes(map[string]map[string]packumentVersionFixture{
		"@acme/core": {"1.1.0": packumentVersion("@acme/core", "1.1.0", "", "")},
	})
	server := fakeNPM(t, packages)
	client := registry.New(server.server.URL)

	resolved, err := client.Resolve(context.Background(), "@acme/core", "^1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Version != "1.1.0" {
		t.Errorf("Version = %q, want 1.1.0", resolved.Version)
	}
	paths := server.requestPaths()
	if len(paths) != 1 || paths[0] != "/@acme%2Fcore" {
		t.Errorf("request path = %v, want [/@acme%%2Fcore] (npm scoped path convention)", paths)
	}
}

func TestRegistryRequestsAbbreviatedPackument(t *testing.T) {
	packages := packumentBytes(map[string]map[string]packumentVersionFixture{
		"lodash": {"1.0.0": packumentVersion("lodash", "1.0.0", "", "")},
	})
	server := fakeNPM(t, packages)
	client := registry.New(server.server.URL)

	if _, err := client.Resolve(context.Background(), "lodash", "^1"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if accept := server.acceptHeader(); !strings.Contains(accept, "application/vnd.npm.install-v2+json") {
		t.Errorf("Accept = %q, want abbreviated packument negotiation", accept)
	}
}

func TestRegistryPackageNotFound(t *testing.T) {
	server := fakeNPM(t, map[string][]byte{})
	client := registry.New(server.server.URL)

	if _, err := client.Resolve(context.Background(), "nope", "^1"); !errors.Is(err, domain.ErrPackageNotFound) {
		t.Fatalf("Resolve error = %v, want ErrPackageNotFound", err)
	}
}

func TestRegistryUnresolvableSpec(t *testing.T) {
	packages := packumentBytes(map[string]map[string]packumentVersionFixture{
		"old": {"1.0.0": packumentVersion("old", "1.0.0", "", "")},
	})
	server := fakeNPM(t, packages)
	client := registry.New(server.server.URL)

	if _, err := client.Resolve(context.Background(), "old", "^9"); !errors.Is(err, domain.ErrUnresolvableSpec) {
		t.Fatalf("Resolve error = %v, want ErrUnresolvableSpec", err)
	}
}

func TestRegistrySkipsYankedEntries(t *testing.T) {
	packages := packumentBytes(map[string]map[string]packumentVersionFixture{
		"flaky": {
			"1.0.0": packumentVersionFixture{Name: "flaky", Version: "1.0.0"}, // no tarball => yanked
			"2.0.0": packumentVersion("flaky", "2.0.0", "", ""),
		},
	})
	server := fakeNPM(t, packages)
	client := registry.New(server.server.URL)

	resolved, err := client.Resolve(context.Background(), "flaky", "^1 || ^2")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0 with yanked 1.0.0 skipped", resolved.Version)
	}
}

func TestRegistryCachesPackument(t *testing.T) {
	packages := packumentBytes(map[string]map[string]packumentVersionFixture{
		"cache": {
			"1.0.0": packumentVersion("cache", "1.0.0", "", ""),
			"2.0.0": packumentVersion("cache", "2.0.0", "", ""),
		},
	})
	server := fakeNPM(t, packages)
	client := registry.New(server.server.URL)
	ctx := context.Background()

	if _, err := client.Resolve(ctx, "cache", "^1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Resolve(ctx, "cache", "^2"); err != nil {
		t.Fatal(err)
	}
	if got := len(server.requestPaths()); got != 1 {
		t.Errorf("packument fetched %d times, want 1 (cached)", got)
	}
}

func TestRegistryRetriesThenFailsOn5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "registry on fire", http.StatusInternalServerError)
	}))
	defer server.Close()
	client := registry.New(server.URL)

	if _, err := client.Resolve(context.Background(), "lodash", "^1"); err == nil {
		t.Fatalf("Resolve should fail after retries, got nil error")
	}
}

func TestRegistryRetriesTransientThenSucceeds(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "flake", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"versions":{"1.0.0":{"name":"flake","version":"1.0.0","dist":{"integrity":"sha512-1","tarball":"https://r.example/flake-1.tgz"}}}}`))
	}))
	defer server.Close()
	client := registry.New(server.URL)

	resolved, err := client.Resolve(context.Background(), "flake", "^1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", resolved.Version)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2 (one retry)", attempts)
	}
}

func TestRegistryRejectsInvalidNameAndSpec(t *testing.T) {
	server := fakeNPM(t, map[string][]byte{})
	client := registry.New(server.server.URL)
	ctx := context.Background()

	if _, err := client.Resolve(ctx, "", "^1"); !errors.Is(err, domain.ErrInvalidDepSpec) {
		t.Errorf("invalid name error = %v, want ErrInvalidDepSpec", err)
	}
	if _, err := client.Resolve(ctx, "lodash", "latest"); !errors.Is(err, domain.ErrInvalidDepSpec) {
		t.Errorf("invalid spec error = %v, want ErrInvalidDepSpec", err)
	}
}
