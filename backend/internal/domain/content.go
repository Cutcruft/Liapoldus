package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Content struct {
	ID           string                    `json:"id"`
	SiteID       string                    `json:"siteId"`
	Key          string                    `json:"-"`
	CollectionID string                    `json:"collectionId"`
	Fields       map[string]any            `json:"fields"`
	Translations map[string]map[string]any `json:"translations"`
	CreatedAt    time.Time                 `json:"createdAt"`
	UpdatedAt    time.Time                 `json:"updatedAt"`
}

// ContentData is the client-facing merged view (base + overlay for a locale).
type ContentData struct {
	ID           string         `json:"id"`
	SiteID       string         `json:"siteId"`
	CollectionID string         `json:"collectionId"`
	Locale       string         `json:"locale"`
	Fields       map[string]any `json:"fields"`
}

type ContentService struct{ repo ContentRepository }

func NewContentService(repo ContentRepository) *ContentService { return &ContentService{repo: repo} }

func (s *ContentService) Create(ctx context.Context, siteID, collectionID, id string, fields map[string]any) (Content, error) {
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		return Content{}, fmt.Errorf("%w: collectionId is required", ErrInvalidRequest)
	}
	if fields == nil {
		fields = map[string]any{}
	}
	if strings.TrimSpace(id) == "" {
		generated, err := NewID("content")
		if err != nil {
			return Content{}, err
		}
		id = generated
	}
	now := time.Now().UTC()
	c := Content{
		ID: id, SiteID: siteID, Key: id, CollectionID: collectionID,
		Fields: fields, Translations: map[string]map[string]any{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateContent(ctx, c); err != nil {
		return Content{}, err
	}
	return c, nil
}

func (s *ContentService) Get(ctx context.Context, siteID, id string) (Content, error) {
	c, err := s.repo.GetContent(ctx, id)
	if err != nil {
		return Content{}, err
	}
	if c.SiteID != siteID {
		return Content{}, fmt.Errorf("%w: content", ErrNotFound)
	}
	return c, nil
}

func (s *ContentService) List(ctx context.Context, siteID, collectionID string) ([]Content, error) {
	return s.repo.ListContentsBySite(ctx, siteID, collectionID)
}

func (s *ContentService) UpdateFields(ctx context.Context, siteID, id string, fields map[string]any) (Content, error) {
	current, err := s.Get(ctx, siteID, id)
	if err != nil {
		return Content{}, err
	}
	current.Fields = fields
	current.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateContent(ctx, current); err != nil {
		return Content{}, err
	}
	return current, nil
}

func (s *ContentService) SetTranslation(ctx context.Context, siteID, id, locale string, fields map[string]any) (Content, error) {
	current, err := s.Get(ctx, siteID, id)
	if err != nil {
		return Content{}, err
	}
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return Content{}, fmt.Errorf("%w: locale is required", ErrInvalidRequest)
	}
	if current.Translations == nil {
		current.Translations = map[string]map[string]any{}
	}
	current.Translations[locale] = fields
	current.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateContent(ctx, current); err != nil {
		return Content{}, err
	}
	return current, nil
}

func (s *ContentService) DeleteTranslation(ctx context.Context, siteID, id, locale string) error {
	current, err := s.Get(ctx, siteID, id)
	if err != nil {
		return err
	}
	delete(current.Translations, locale)
	current.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateContent(ctx, current)
}

func (s *ContentService) Delete(ctx context.Context, siteID, id string) error {
	if _, err := s.Get(ctx, siteID, id); err != nil {
		return err
	}
	return s.repo.DeleteContent(ctx, id)
}

// GetMerged returns the client view: base fields overlaid with translations
// for the locale; missing translation falls back to base entirely.
func (s *ContentService) GetMerged(ctx context.Context, siteID, id, locale string) (ContentData, error) {
	c, err := s.Get(ctx, siteID, id)
	if err != nil {
		return ContentData{}, err
	}
	return merged(c, locale), nil
}

// Batch returns merged client views by ids; missing ids are skipped.
func (s *ContentService) Batch(ctx context.Context, siteID string, ids []string, locale string) (map[string]ContentData, error) {
	byID, err := s.repo.GetContentsByIDs(ctx, siteID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]ContentData, len(byID))
	for id, c := range byID {
		out[id] = merged(c, locale)
	}
	return out, nil
}

func merged(c Content, locale string) ContentData {
	fields := make(map[string]any, len(c.Fields))
	for k, v := range c.Fields {
		fields[k] = v
	}
	if locale != "" {
		for k, v := range c.Translations[locale] {
			fields[k] = v
		}
	}
	return ContentData{ID: c.ID, SiteID: c.SiteID, CollectionID: c.CollectionID, Locale: locale, Fields: fields}
}
