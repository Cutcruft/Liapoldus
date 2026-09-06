package unit

import (
	"context"
	"sync"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/deps"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// fakeRegistry replaces the real npm registry client in unit tests. A call
// returns whatever the resolve hook computes; calls are counted.
type fakeRegistry struct {
	mu      sync.Mutex
	calls   int
	resolve func(ctx context.Context, name, spec string) (deps.ResolvedVersion, error)
}

func (f *fakeRegistry) Resolve(ctx context.Context, name, spec string) (deps.ResolvedVersion, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.resolve == nil {
		return deps.ResolvedVersion{}, nil
	}
	return f.resolve(ctx, name, spec)
}

func (f *fakeRegistry) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newTestDepsService(t *testing.T, reg deps.Registry) (*deps.Service, domain.Storage) {
	t.Helper()
	db := storage.NewMemory()
	return deps.NewService(db, db, reg), db
}
