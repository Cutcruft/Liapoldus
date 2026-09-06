package unit

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

func TestSetCacheConfigValidatesLimit(t *testing.T) {
	svc, _ := newTestDepsService(t, &fakeRegistry{})
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		val  int64
	}{
		{"zero", 0},
		{"negative", -1},
		{"over-cap", domain.MaxDepsCacheBytes + 1},
	} {
		if err := svc.SetCacheConfig(ctx, "site-1", tc.val); !errors.Is(err, domain.ErrInvalidCacheConfig) {
			t.Errorf("SetCacheConfig(%s) error = %v, want ErrInvalidCacheConfig", tc.name, err)
		}
	}

	// The invalid writes must not have left a config behind.
	if _, found, err := svc.GetCacheConfig(ctx, "site-1"); err != nil || found {
		t.Errorf("GetCacheConfig after invalid writes = (found=%v, err=%v), want (false, nil)", found, err)
	}
}

func TestGetCacheConfigRoundTrip(t *testing.T) {
	svc, _ := newTestDepsService(t, &fakeRegistry{})
	ctx := context.Background()

	if _, found, err := svc.GetCacheConfig(ctx, "site-1"); err != nil || found {
		t.Fatalf("initial GetCacheConfig = (found=%v, err=%v), want (false, nil)", found, err)
	}

	if err := svc.SetCacheConfig(ctx, "site-1", 1<<30); err != nil {
		t.Fatalf("SetCacheConfig: %v", err)
	}
	cfg, found, err := svc.GetCacheConfig(ctx, "site-1")
	if err != nil || !found {
		t.Fatalf("GetCacheConfig after set = (found=%v, err=%v), want (true, nil)", found, err)
	}
	if cfg.SiteID != "site-1" || cfg.MaxDepsBytes != 1<<30 {
		t.Errorf("cfg = %+v", cfg)
	}

	// Unrelated sites stay unconfigured.
	if _, found, err := svc.GetCacheConfig(ctx, "site-2"); err != nil || found {
		t.Errorf("GetCacheConfig(site-2) = (found=%v, err=%v), want (false, nil)", found, err)
	}
}

func TestEffectiveCacheLimitIsMaxAcrossSites(t *testing.T) {
	svc, _ := newTestDepsService(t, &fakeRegistry{})
	ctx := context.Background()

	if _, limited, err := svc.EffectiveCacheLimit(ctx); err != nil || limited {
		t.Fatalf("no config: (limited=%v, err=%v), want (false, nil)", limited, err)
	}

	_ = svc.SetCacheConfig(ctx, "site-1", 1<<30)
	_ = svc.SetCacheConfig(ctx, "site-2", 4<<30)

	got, limited, err := svc.EffectiveCacheLimit(ctx)
	if err != nil || !limited {
		t.Fatalf("EffectiveCacheLimit = (limited=%v, err=%v), want (true, nil)", limited, err)
	}
	if got != 4<<30 {
		t.Errorf("EffectiveCacheLimit = %d, want %d", got, 4<<30)
	}
}

// fakeTarballCache is an in-memory stand-in for the on-disk tarball store,
// tracking sizes and deletions so eviction is observable without touching the
// filesystem.
type fakeTarballCache struct {
	mm    map[string]int64 // "name\x00version" -> size
	order []string
}

func newFakeTarballCache() *fakeTarballCache {
	return &fakeTarballCache{mm: map[string]int64{}}
}

func (f *fakeTarballCache) key(name, version string) string { return name + "\x00" + version }

func (f *fakeTarballCache) put(name, version string, size int64) {
	key := f.key(name, version)
	if _, ok := f.mm[key]; !ok {
		f.order = append(f.order, key)
	}
	f.mm[key] = size
}

func (f *fakeTarballCache) Has(name, version string) bool {
	_, ok := f.mm[f.key(name, version)]
	return ok && f.mm[f.key(name, version)] > 0
}

func (f *fakeTarballCache) Size(name, version string) int64 {
	return f.mm[f.key(name, version)]
}

func (f *fakeTarballCache) Delete(name, version string) error {
	f.mm[f.key(name, version)] = 0
	return nil
}

func TestEvictTarballsLRU(t *testing.T) {
	svc, db := newTestDepsService(t, &fakeRegistry{})
	cache := newFakeTarballCache()
	svc.WithTarballCache(cache)
	ctx := context.Background()

	// Three cached tarballs; total 90 bytes. Oldest is "a", newest "c".
	cache.put("a", "1.0.0", 30)
	cache.put("b", "1.0.0", 30)
	cache.put("c", "1.0.0", 30)
	// Access stamps are recorded on reads during builds; seed them oldest-first.
	_ = db.TouchTarballAccess(ctx, "a", "1.0.0")
	time.Sleep(2 * time.Millisecond)
	_ = db.TouchTarballAccess(ctx, "b", "1.0.0")
	time.Sleep(2 * time.Millisecond)
	_ = db.TouchTarballAccess(ctx, "c", "1.0.0")

	// Limit 60 -> must drop the oldest 30 ("a") to reach 60.
	if err := svc.SetCacheConfig(ctx, "site-1", 60); err != nil {
		t.Fatalf("SetCacheConfig: %v", err)
	}
	evictedBytes, evicted, err := svc.EvictTarballs(ctx)
	if err != nil {
		t.Fatalf("EvictTarballs: %v", err)
	}
	if evicted != 1 || evictedBytes != 30 {
		t.Errorf("evict = (%d bytes, %d files), want (30, 1)", evictedBytes, evicted)
	}
	if cache.Size("a", "1.0.0") != 0 {
		t.Errorf("oldest 'a' should be evicted")
	}
	if cache.Size("b", "1.0.0") == 0 || cache.Size("c", "1.0.0") == 0 {
		t.Errorf("newer 'b'/'c' should be retained")
	}
}

func TestEvictTarballsUnlimitedNoop(t *testing.T) {
	svc, _ := newTestDepsService(t, &fakeRegistry{})
	cache := newFakeTarballCache()
	cache.put("a", "1.0.0", 100)
	svc.WithTarballCache(cache)

	// No config yet -> unlimited -> no-op.
	got, n, err := svc.EvictTarballs(context.Background())
	if err != nil || n != 0 || got != 0 {
		t.Errorf("unlimited evict = (%d bytes, %d files, err=%v), want (0, 0, nil)", got, n, err)
	}
	if cache.Size("a", "1.0.0") == 0 {
		t.Errorf("no-op must not delete tarballs")
	}
}

func TestEvictTarballsWithoutStoreNoop(t *testing.T) {
	svc, _ := newTestDepsService(t, &fakeRegistry{})
	ctx := context.Background()
	_ = svc.SetCacheConfig(ctx, "site-1", 1<<20)
	// tarballs is nil -> eviction must be a safe no-op.
	got, n, err := svc.EvictTarballs(ctx)
	if err != nil || n != 0 || got != 0 {
		t.Errorf("no-store evict = (%d, %d, %v), want (0, 0, nil)", got, n, err)
	}
}

func TestCacheConfigAdminCRUD(t *testing.T) {
	app, _, siteID := newDepsApp(t, &fakeRegistry{})
	handler := admin.NewRouter(app)

	// Unset -> 0 (no limit of its own).
	get := request(t, handler, http.MethodGet, "/api/sites/"+siteID+"/cache-config", nil)
	if get.Code != http.StatusOK {
		t.Fatalf("GET cache-config status = %d (%s)", get.Code, get.Body.String())
	}
	var got struct {
		MaxDepsBytes int64 `json:"maxDepsBytes"`
	}
	decodeResponse(t, get, &got)
	if got.MaxDepsBytes != 0 {
		t.Errorf("initial maxDepsBytes = %d, want 0", got.MaxDepsBytes)
	}

	// Set a limit.
	put := request(t, handler, http.MethodPut, "/api/sites/"+siteID+"/cache-config", map[string]any{"maxDepsBytes": 1 << 30})
	if put.Code != http.StatusOK {
		t.Fatalf("PUT cache-config status = %d (%s)", put.Code, put.Body.String())
	}
	decodeResponse(t, put, &got)
	if got.MaxDepsBytes != 1<<30 {
		t.Errorf("PUT maxDepsBytes = %d, want %d", got.MaxDepsBytes, 1<<30)
	}

	// Read back.
	get = request(t, handler, http.MethodGet, "/api/sites/"+siteID+"/cache-config", nil)
	decodeResponse(t, get, &got)
	if got.MaxDepsBytes != 1<<30 {
		t.Errorf("GET after PUT maxDepsBytes = %d, want %d", got.MaxDepsBytes, 1<<30)
	}

	// Invalid -> 400.
	bad := request(t, handler, http.MethodPut, "/api/sites/"+siteID+"/cache-config", map[string]any{"maxDepsBytes": 0})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("PUT cache-config(0) status = %d, want 400", bad.Code)
	}
	over := request(t, handler, http.MethodPut, "/api/sites/"+siteID+"/cache-config", map[string]any{"maxDepsBytes": 1 << 40})
	if over.Code != http.StatusBadRequest {
		t.Fatalf("PUT cache-config(over-cap) status = %d, want 400", over.Code)
	}
}
