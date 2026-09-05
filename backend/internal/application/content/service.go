package content

import (
	"context"
	"fmt"
	"strings"
	"time"

	idgen "github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type Service struct{ repo domain.ContentRepository }

func NewService(repo domain.ContentRepository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, siteID, collectionID, id string, fields map[string]any) (domain.Content, error) {
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		return domain.Content{}, fmt.Errorf("%w: collectionId is required", domain.ErrInvalidRequest)
	}
	if fields == nil {
		fields = map[string]any{}
	}
	if strings.TrimSpace(id) == "" {
		generated, err := idgen.New(idgen.Content)
		if err != nil {
			return domain.Content{}, err
		}
		id = generated
	}
	now := time.Now().UTC()
	c := domain.Content{
		ID: id, SiteID: siteID, Key: id, CollectionID: collectionID,
		Fields: fields, Translations: map[string]map[string]any{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateContent(ctx, c); err != nil {
		return domain.Content{}, err
	}
	return c, nil
}

func (s *Service) Get(ctx context.Context, siteID, id string) (domain.Content, error) {
	c, err := s.repo.GetContent(ctx, id)
	if err != nil {
		return domain.Content{}, err
	}
	if c.SiteID != siteID {
		return domain.Content{}, fmt.Errorf("%w: content", domain.ErrNotFound)
	}
	return c, nil
}

func (s *Service) List(ctx context.Context, siteID, collectionID string) ([]domain.Content, error) {
	return s.repo.ListContentsBySite(ctx, siteID, collectionID)
}

func (s *Service) UpdateFields(ctx context.Context, siteID, id string, fields map[string]any) (domain.Content, error) {
	current, err := s.Get(ctx, siteID, id)
	if err != nil {
		return domain.Content{}, err
	}
	current.Fields = fields
	current.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateContent(ctx, current); err != nil {
		return domain.Content{}, err
	}
	return current, nil
}

func (s *Service) SetTranslation(ctx context.Context, siteID, id, locale string, fields map[string]any) (domain.Content, error) {
	current, err := s.Get(ctx, siteID, id)
	if err != nil {
		return domain.Content{}, err
	}
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return domain.Content{}, fmt.Errorf("%w: locale is required", domain.ErrInvalidRequest)
	}
	if current.Translations == nil {
		current.Translations = map[string]map[string]any{}
	}
	current.Translations[locale] = fields
	current.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateContent(ctx, current); err != nil {
		return domain.Content{}, err
	}
	return current, nil
}

func (s *Service) DeleteTranslation(ctx context.Context, siteID, id, locale string) error {
	current, err := s.Get(ctx, siteID, id)
	if err != nil {
		return err
	}
	delete(current.Translations, locale)
	current.UpdatedAt = time.Now().UTC()
	return s.repo.UpdateContent(ctx, current)
}

func (s *Service) Delete(ctx context.Context, siteID, id string) error {
	if _, err := s.Get(ctx, siteID, id); err != nil {
		return err
	}
	return s.repo.DeleteContent(ctx, id)
}

// GetMerged returns the client view: base fields overlaid with translations
// for the locale; missing translation falls back to base entirely.
func (s *Service) GetMerged(ctx context.Context, siteID, id, locale string) (domain.ContentData, error) {
	c, err := s.Get(ctx, siteID, id)
	if err != nil {
		return domain.ContentData{}, err
	}
	return merged(c, locale), nil
}

// Batch returns merged client views by ids; missing ids are skipped.
func (s *Service) Batch(ctx context.Context, siteID string, ids []string, locale string) (map[string]domain.ContentData, error) {
	byID, err := s.repo.GetContentsByIDs(ctx, siteID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]domain.ContentData, len(byID))
	for id2, c := range byID {
		out[id2] = merged(c, locale)
	}
	return out, nil
}

func merged(c domain.Content, locale string) domain.ContentData {
	fields := make(map[string]any, len(c.Fields))
	for k, v := range c.Fields {
		fields[k] = v
	}
	if locale != "" {
		for k, v := range c.Translations[locale] {
			fields[k] = v
		}
	}
	return domain.ContentData{ID: c.ID, SiteID: c.SiteID, CollectionID: c.CollectionID, Locale: locale, Fields: fields}
}
