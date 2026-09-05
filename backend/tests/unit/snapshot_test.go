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

func TestSnapshotServiceCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().ListPagesBySite(gomock.Any(), "site_1").Return([]domain.Page{
		{ID: "page_1", SiteID: "site_1"},
		{ID: "page_2", SiteID: "site_1"},
	}, nil)
	pageRepo.EXPECT().ListPageVersions(gomock.Any(), "page_1").Return([]domain.PageVersion{
		{ID: "pagever_v1", PageID: "page_1", Number: 1},
		{ID: "pagever_v2", PageID: "page_1", Number: 2},
	}, nil)
	pageRepo.EXPECT().ListPageVersions(gomock.Any(), "page_2").Return([]domain.PageVersion{
		{ID: "pagever_p2", PageID: "page_2", Number: 1},
	}, nil)

	snapshotRepo := mocks.NewMockSnapshotRepository(ctrl)
	snapshotRepo.EXPECT().CreateSnapshot(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, snapshot domain.Snapshot) error {
			if !strings.HasPrefix(snapshot.ID, "snapshot_") {
				t.Fatalf("snapshot id = %q, want snapshot_ prefix", snapshot.ID)
			}
			if len(snapshot.Pages) != 2 {
				t.Fatalf("snapshot pages = %#v", snapshot.Pages)
			}
			if snapshot.Pages[0].PageID != "page_1" || snapshot.Pages[0].VersionID != "pagever_v2" || snapshot.Pages[0].Version != 2 {
				t.Fatalf("snapshot page 0 = %#v, want latest version", snapshot.Pages[0])
			}
			return nil
		})

	snapshot, err := domain.NewSnapshotService(siteRepo, pageRepo, snapshotRepo).Create(context.Background(), "site_1", "Release 1")
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if len(snapshot.Pages) != 2 || snapshot.Pages[0].VersionID != "pagever_v2" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestSnapshotServiceCreateSiteNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_missing").Return(domain.Site{}, domain.ErrNotFound)

	pageRepo := mocks.NewMockPageRepository(ctrl)
	snapshotRepo := mocks.NewMockSnapshotRepository(ctrl)

	_, err := domain.NewSnapshotService(siteRepo, pageRepo, snapshotRepo).Create(context.Background(), "site_missing", "Release 1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestSnapshotServiceCreateMissingName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	pageRepo := mocks.NewMockPageRepository(ctrl)
	snapshotRepo := mocks.NewMockSnapshotRepository(ctrl)

	_, err := domain.NewSnapshotService(siteRepo, pageRepo, snapshotRepo).Create(context.Background(), "site_1", "  ")
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestSnapshotServiceCreatePageWithoutVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	pageRepo := mocks.NewMockPageRepository(ctrl)
	pageRepo.EXPECT().ListPagesBySite(gomock.Any(), "site_1").Return([]domain.Page{
		{ID: "page_1", SiteID: "site_1"},
	}, nil)
	pageRepo.EXPECT().ListPageVersions(gomock.Any(), "page_1").Return(nil, nil)

	snapshotRepo := mocks.NewMockSnapshotRepository(ctrl)

	_, err := domain.NewSnapshotService(siteRepo, pageRepo, snapshotRepo).Create(context.Background(), "site_1", "Release 1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}
