package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/deploy"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/tests/unit/mocks"
	"go.uber.org/mock/gomock"
)

type fakeReleaser struct {
	createFn func(ctx context.Context, siteID, snapshotID, environment string) (domain.Build, error)
}

func (f *fakeReleaser) Create(ctx context.Context, siteID, snapshotID, environment string) (domain.Build, error) {
	return f.createFn(ctx, siteID, snapshotID, environment)
}

func TestDeployServiceRelease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	snapshotRepo := mocks.NewMockSnapshotRepository(ctrl)
	snapshotRepo.EXPECT().GetSnapshot(gomock.Any(), "snap_new").Return(domain.Snapshot{ID: "snap_new", SiteID: "site_1", CreatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}, nil)

	deployRepo := mocks.NewMockDeploymentRepository(ctrl)
	deployRepo.EXPECT().GetDeployment(gomock.Any(), "site_1", "production").Return(domain.Deployment{}, domain.ErrNotFound)
	deployRepo.EXPECT().SetDeployment(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, d domain.Deployment) error {
		if d.SnapshotID != "snap_new" {
			t.Fatalf("deployment snapshot = %q, want snap_new", d.SnapshotID)
		}
		return nil
	})

	var buildCreated bool
	builds := &fakeReleaser{
		createFn: func(_ context.Context, siteID, snapshotID, environment string) (domain.Build, error) {
			if siteID != "site_1" || snapshotID != "snap_new" || environment != "production" {
				t.Fatalf("build.Create(%s, %s, %s)", siteID, snapshotID, environment)
			}
			buildCreated = true
			return domain.Build{ID: "build_1"}, nil
		},
	}

	svc := deploy.NewService(siteRepo, snapshotRepo, deployRepo, builds)

	dep, err := svc.Release(context.Background(), "site_1", "snap_new", "production")
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if !buildCreated {
		t.Fatal("build was not created")
	}
	if dep.SnapshotID != "snap_new" {
		t.Fatalf("dep.SnapshotID = %q", dep.SnapshotID)
	}
}

func TestDeployServiceReleaseIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	snapshotRepo := mocks.NewMockSnapshotRepository(ctrl)
	snapshotRepo.EXPECT().GetSnapshot(gomock.Any(), "snap_new").Return(domain.Snapshot{ID: "snap_new", SiteID: "site_1"}, nil)

	deployRepo := mocks.NewMockDeploymentRepository(ctrl)
	existing := domain.Deployment{ID: "dep_1", SiteID: "site_1", SnapshotID: "snap_new", Environment: "production"}
	deployRepo.EXPECT().GetDeployment(gomock.Any(), "site_1", "production").Return(existing, nil)

	builds := &fakeReleaser{
		createFn: func(_ context.Context, _, _, _ string) (domain.Build, error) {
			t.Fatal("build should not be created for idempotent release")
			return domain.Build{}, errors.New("unreachable")
		},
	}

	svc := deploy.NewService(siteRepo, snapshotRepo, deployRepo, builds)

	dep, err := svc.Release(context.Background(), "site_1", "snap_new", "production")
	if err != nil {
		t.Fatalf("release idempotent: %v", err)
	}
	if dep.ID != "dep_1" {
		t.Fatalf("dep.ID = %q, want dep_1", dep.ID)
	}
}

func TestDeployServiceReleaseInvalidEnv(t *testing.T) {
	svc := deploy.NewService(nil, nil, nil, nil)
	_, err := svc.Release(context.Background(), "site_1", "snap_1", "staging")
	if err == nil {
		t.Fatal("expected error for invalid environment")
	}
}

func TestDeployServiceRollbackEarlier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	snapshotRepo := mocks.NewMockSnapshotRepository(ctrl)
	snapshotRepo.EXPECT().GetSnapshot(gomock.Any(), "snap_old").Return(domain.Snapshot{ID: "snap_old", SiteID: "site_1", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}, nil)
	snapshotRepo.EXPECT().GetSnapshot(gomock.Any(), "snap_new").Return(domain.Snapshot{ID: "snap_new", SiteID: "site_1", CreatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}, nil)

	deployRepo := mocks.NewMockDeploymentRepository(ctrl)
	current := domain.Deployment{ID: "dep_1", SiteID: "site_1", SnapshotID: "snap_new", Environment: "production"}
	deployRepo.EXPECT().GetDeployment(gomock.Any(), "site_1", "production").Return(current, nil)
	deployRepo.EXPECT().SetDeployment(gomock.Any(), gomock.Any()).Return(nil)

	builds := &fakeReleaser{
		createFn: func(_ context.Context, siteID, snapshotID, environment string) (domain.Build, error) {
			if snapshotID != "snap_old" {
				t.Fatalf("build snapshot = %q, want snap_old", snapshotID)
			}
			return domain.Build{}, nil
		},
	}

	svc := deploy.NewService(siteRepo, snapshotRepo, deployRepo, builds)

	dep, err := svc.Rollback(context.Background(), "site_1", "snap_old", "production")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if dep.SnapshotID != "snap_old" {
		t.Fatalf("dep.SnapshotID = %q, want snap_old", dep.SnapshotID)
	}
}

func TestDeployServiceRollbackRejectsForward(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	siteRepo := mocks.NewMockSiteRepository(ctrl)
	siteRepo.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	snapshotRepo := mocks.NewMockSnapshotRepository(ctrl)
	snapshotRepo.EXPECT().GetSnapshot(gomock.Any(), "snap_old").Return(domain.Snapshot{ID: "snap_old", SiteID: "site_1", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}, nil)
	snapshotRepo.EXPECT().GetSnapshot(gomock.Any(), "snap_new").Return(domain.Snapshot{ID: "snap_new", SiteID: "site_1", CreatedAt: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)}, nil)

	deployRepo := mocks.NewMockDeploymentRepository(ctrl)
	current := domain.Deployment{ID: "dep_1", SiteID: "site_1", SnapshotID: "snap_old", Environment: "production"}
	deployRepo.EXPECT().GetDeployment(gomock.Any(), "site_1", "production").Return(current, nil)

	svc := deploy.NewService(siteRepo, snapshotRepo, deployRepo, nil)
	_, err := svc.Rollback(context.Background(), "site_1", "snap_new", "production")
	if err == nil {
		t.Fatal("expected error when rollback target is not earlier")
	}
}

func TestDeployServiceActiveNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployRepo := mocks.NewMockDeploymentRepository(ctrl)
	deployRepo.EXPECT().GetDeployment(gomock.Any(), "site_1", "production").Return(domain.Deployment{}, domain.ErrNotFound)

	svc := deploy.NewService(nil, nil, deployRepo, nil)
	_, err := svc.Active(context.Background(), "site_1", "production")
	if err == nil {
		t.Fatal("expected ErrNotFound")
	}
}

func TestDeployServiceList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deployRepo := mocks.NewMockDeploymentRepository(ctrl)
	deployRepo.EXPECT().ListDeploymentsBySite(gomock.Any(), "site_1").Return([]domain.Deployment{
		{ID: "dep_1", SiteID: "site_1", Environment: "development", SnapshotID: "snap_dev"},
		{ID: "dep_2", SiteID: "site_1", Environment: "production", SnapshotID: "snap_prod"},
	}, nil)

	svc := deploy.NewService(nil, nil, deployRepo, nil)
	deployments, err := svc.ListBySite(context.Background(), "site_1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(deployments) != 2 {
		t.Fatalf("got %d deployments, want 2", len(deployments))
	}
}
