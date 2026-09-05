package unit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	assetapp "github.com/liapoldus/liapoldus/backend/internal/application/asset"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/tests/unit/mocks"
	"go.uber.org/mock/gomock"
)

func newAssetService(repo domain.AssetRepository, blobs domain.AssetBlobStore, sites domain.SiteRepository) *assetapp.Service {
	return assetapp.NewService(repo, blobs, sites, assetapp.Settings{
		MasterVariant: "master",
		FallbackName:  "asset",
		FallbackMime:  "application/octet-stream",
		URLTemplate:   "/files/{id}",
	})
}

type fakeBlobStore struct {
	blobs map[string][]byte
}

func newFakeBlobStore() *fakeBlobStore { return &fakeBlobStore{blobs: map[string][]byte{}} }

func (f *fakeBlobStore) Save(_ context.Context, siteID, id string, data io.Reader) (int64, string, error) {
	b, err := io.ReadAll(data)
	if err != nil {
		return 0, "", err
	}
	f.blobs[siteID+"/"+id] = b
	return int64(len(b)), fmt.Sprintf("%q", b), nil
}

func (f *fakeBlobStore) Open(_ context.Context, siteID, id string) (io.ReadCloser, error) {
	b, ok := f.blobs[siteID+"/"+id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (f *fakeBlobStore) Delete(_ context.Context, siteID, id string) error {
	delete(f.blobs, siteID+"/"+id)
	return nil
}

func TestAssetServiceCreateMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sites := mocks.NewMockSiteRepository(ctrl)
	sites.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	blobs := newFakeBlobStore()
	repo := mocks.NewMockAssetRepository(ctrl)
	repo.EXPECT().CreateAsset(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, a domain.Asset) error {
			if !strings.HasPrefix(a.ID, "asset_") {
				t.Fatalf("asset id = %q, want asset_ prefix", a.ID)
			}
			if a.Mime != "text/plain" || a.Size != 5 {
				t.Fatalf("asset = %#v", a)
			}
			return nil
		})

	service := newAssetService(repo, blobs, sites)
	a, err := service.Create(context.Background(), "site_1", "note.txt", "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}

	meta := service.Metadata(a)
	if len(meta.Variants) != 1 || meta.Variants[0].Name != "master" || meta.Variants[0].URL != "/files/"+a.ID {
		t.Fatalf("metadata = %#v", meta)
	}
	if !service.IsMaster("master") || service.IsMaster("webp") {
		t.Fatalf("IsMaster behavior wrong")
	}
	if got := service.MetadataList([]domain.Asset{a}); len(got) != 1 {
		t.Fatalf("metadata list = %#v", got)
	}
}

func TestAssetServiceCreateFallsBackToDefaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sites := mocks.NewMockSiteRepository(ctrl)
	sites.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	blobs := newFakeBlobStore()
	repo := mocks.NewMockAssetRepository(ctrl)
	repo.EXPECT().CreateAsset(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, a domain.Asset) error {
			if a.Name != "asset" || a.Mime != "application/octet-stream" {
				t.Fatalf("asset = %#v, want settings fallbacks", a)
			}
			return nil
		})

	a, err := newAssetService(repo, blobs, sites).Create(context.Background(), "site_1", "  ", "", strings.NewReader("x"))
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if a.Mime != "application/octet-stream" {
		t.Fatalf("asset = %#v", a)
	}
}

func TestAssetServiceCreateRollsBackBlobOnRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sites := mocks.NewMockSiteRepository(ctrl)
	sites.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	blobs := newFakeBlobStore()
	repo := mocks.NewMockAssetRepository(ctrl)
	repo.EXPECT().CreateAsset(gomock.Any(), gomock.Any()).Return(domain.ErrAlreadyExists)

	_, err := newAssetService(repo, blobs, sites).Create(context.Background(), "site_1", "note", "text/plain", strings.NewReader("hello"))
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("error = %v, want ErrAlreadyExists", err)
	}
	if len(blobs.blobs) != 0 {
		t.Fatalf("blobs = %v, want rollback", blobs.blobs)
	}
}

func TestAssetServiceOpenAndDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sites := mocks.NewMockSiteRepository(ctrl)
	blobs := newFakeBlobStore()
	repo := mocks.NewMockAssetRepository(ctrl)

	asset := domain.Asset{ID: "asset_1", SiteID: "site_1", Mime: "text/plain"}
	blobs.blobs["site_1/asset_1"] = []byte("data")

	repo.EXPECT().GetAsset(gomock.Any(), "asset_1").Return(asset, nil)
	service := newAssetService(repo, blobs, sites)
	opened, reader, err := service.Open(context.Background(), "asset_1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	b, _ := io.ReadAll(reader)
	reader.Close()
	if opened.ID != asset.ID || string(b) != "data" {
		t.Fatalf("opened = %#v, bytes = %q", opened, b)
	}
}
