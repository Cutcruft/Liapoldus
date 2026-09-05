package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/tests/unit/mocks"
	"go.uber.org/mock/gomock"
)

func TestSiteServiceCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSiteRepository(ctrl)
	repo.EXPECT().CreateSite(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, site domain.Site) error {
			if !strings.HasPrefix(site.ID, "site_") {
				t.Fatalf("site id = %q, want site_ prefix", site.ID)
			}
			if site.Name != "Demo" || site.Slug != "demo" {
				t.Fatalf("site = %#v", site)
			}
			return nil
		})

	site, err := domain.NewSiteService(repo).Create(context.Background(), "Demo", "demo")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if site.Name != "Demo" || site.Slug != "demo" {
		t.Fatalf("site = %#v", site)
	}
}

func TestSiteServiceCreateValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSiteRepository(ctrl)

	_, err := domain.NewSiteService(repo).Create(context.Background(), "  ", "  ")
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestSiteServiceCreateRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSiteRepository(ctrl)
	repo.EXPECT().CreateSite(gomock.Any(), gomock.Any()).Return(domain.ErrAlreadyExists)

	_, err := domain.NewSiteService(repo).Create(context.Background(), "Demo", "demo")
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("error = %v, want ErrAlreadyExists", err)
	}
}

func TestSiteServiceGet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	want := domain.Site{ID: "site_1", Name: "Demo", Slug: "demo"}
	repo := mocks.NewMockSiteRepository(ctrl)
	repo.EXPECT().GetSite(gomock.Any(), "site_1").Return(want, nil)

	got, err := domain.NewSiteService(repo).Get(context.Background(), "site_1")
	if err != nil {
		t.Fatalf("get site: %v", err)
	}
	if got != want {
		t.Fatalf("site = %#v, want %#v", got, want)
	}
}

func TestSiteServiceGetNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockSiteRepository(ctrl)
	repo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{}, domain.ErrNotFound)

	_, err := domain.NewSiteService(repo).Get(context.Background(), "site_1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}
