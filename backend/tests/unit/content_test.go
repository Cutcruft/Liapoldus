package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	contentapp "github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/tests/unit/mocks"
	"go.uber.org/mock/gomock"
)

func TestContentServiceCreateGeneratesID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockContentRepository(ctrl)
	repo.EXPECT().CreateContent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, c domain.Content) error {
			if !strings.HasPrefix(c.ID, "content_") {
				t.Fatalf("id = %q, want content_ prefix", c.ID)
			}
			if c.CollectionID != "col.articles" || c.SiteID != "site_1" {
				t.Fatalf("content = %#v", c)
			}
			return nil
		})

	service := contentapp.NewService(repo)
	c, err := service.Create(context.Background(), "site_1", "col.articles", "", map[string]any{"title": "Hi"})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}
	if c.CollectionID != "col.articles" || c.ID == "" {
		t.Fatalf("content = %#v", c)
	}
}

func TestContentServiceCreateRequiresCollection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockContentRepository(ctrl)

	_, err := contentapp.NewService(repo).Create(context.Background(), "site_1", "  ", "", nil)
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestContentServiceGetMerged(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	base := domain.Content{ID: "content_1", SiteID: "site_1", CollectionID: "col", Fields: map[string]any{"title": "Base", "note": "N"}, Translations: map[string]map[string]any{"ru": {"title": "Привет"}}}
	repo := mocks.NewMockContentRepository(ctrl)
	repo.EXPECT().GetContent(gomock.Any(), "content_1").Return(base, nil)

	merged, err := contentapp.NewService(repo).GetMerged(context.Background(), "site_1", "content_1", "ru")
	if err != nil {
		t.Fatalf("get merged: %v", err)
	}
	if merged.Locale != "ru" || merged.Fields["title"] != "Привет" || merged.Fields["note"] != "N" {
		t.Fatalf("merged = %#v", merged)
	}
}

func TestContentServiceGetScopedToSite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockContentRepository(ctrl)
	repo.EXPECT().GetContent(gomock.Any(), "content_1").Return(domain.Content{ID: "content_1", SiteID: "site_other"}, nil)

	_, err := contentapp.NewService(repo).Get(context.Background(), "site_1", "content_1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestContentServiceBatchSkipsMissingIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockContentRepository(ctrl)
	repo.EXPECT().GetContentsByIDs(gomock.Any(), "site_1", gomock.Any()).Return(map[string]domain.Content{
		"content_1": {ID: "content_1", SiteID: "site_1", Fields: map[string]any{"title": "A"}},
	}, nil)

	out, err := contentapp.NewService(repo).Batch(context.Background(), "site_1", []string{"content_1", "content_2"}, "")
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("batch = %#v, want only existing id", out)
	}
}
