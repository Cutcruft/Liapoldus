package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/tests/unit/mocks"
	"go.uber.org/mock/gomock"
)

func newSiteService(repo domain.SiteRepository) *site.Service {
	return site.NewService(repo, site.Settings{DefaultLocale: "ru"})
}

func TestSiteServiceCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSiteRepository(ctrl)
	repo.EXPECT().CreateSite(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, created domain.Site) error {
			if !strings.HasPrefix(created.ID, "site_") {
				t.Fatalf("site id = %q, want site_ prefix", created.ID)
			}
			if created.Name != "Demo" || created.Slug != "demo" || created.DefaultLocale != "ru" {
				t.Fatalf("site = %#v", created)
			}
			return nil
		})

	site, err := newSiteService(repo).Create(context.Background(), "Demo", "demo", "ru", nil)
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if site.Name != "Demo" || site.Slug != "demo" {
		t.Fatalf("site = %#v", site)
	}
}

func TestSiteServiceCreateLocaleFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSiteRepository(ctrl)
	repo.EXPECT().CreateSite(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, created domain.Site) error {
			if created.DefaultLocale != "ru" {
				t.Fatalf("default locale = %q, want settings fallback", created.DefaultLocale)
			}
			return nil
		})

	_, err := newSiteService(repo).Create(context.Background(), "Demo", "demo", "", nil)
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
}

func TestSiteServiceCreateValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSiteRepository(ctrl)

	_, err := newSiteService(repo).Create(context.Background(), "  ", "  ", "", nil)
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestSiteServiceCreateRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSiteRepository(ctrl)
	repo.EXPECT().CreateSite(gomock.Any(), gomock.Any()).Return(domain.ErrAlreadyExists)

	_, err := newSiteService(repo).Create(context.Background(), "Demo", "demo", "ru", nil)
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("error = %v, want ErrAlreadyExists", err)
	}
}

func TestSiteServiceGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	want := domain.Site{ID: "site_1", Name: "Demo", Slug: "demo", DefaultLocale: "ru"}
	repo := mocks.NewMockSiteRepository(ctrl)
	repo.EXPECT().GetSite(gomock.Any(), "site_1").Return(want, nil)

	got, err := newSiteService(repo).Get(context.Background(), "site_1")
	if err != nil {
		t.Fatalf("get site: %v", err)
	}
	if got.ID != want.ID || got.Name != want.Name || got.Slug != want.Slug || got.DefaultLocale != want.DefaultLocale {
		t.Fatalf("site = %#v, want %#v", got, want)
	}
}

func TestSiteServiceGetNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSiteRepository(ctrl)
	repo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{}, domain.ErrNotFound)

	_, err := newSiteService(repo).Get(context.Background(), "site_1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}
