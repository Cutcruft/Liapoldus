package form

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Settings carries the email validation pattern used for "email" form fields.
type Settings struct {
	EmailPattern *regexp.Regexp
}

type Service struct {
	repo     domain.FormRepository
	sites    domain.SiteRepository
	settings Settings
}

func NewService(repo domain.FormRepository, sites domain.SiteRepository, settings Settings) *Service {
	return &Service{repo: repo, sites: sites, settings: settings}
}

func (s *Service) Create(ctx context.Context, siteID, name string, definition map[string]any) (domain.Form, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return domain.Form{}, err
	}
	if name == "" {
		return domain.Form{}, fmt.Errorf("%w: name is required", domain.ErrInvalidRequest)
	}
	if definition == nil {
		return domain.Form{}, fmt.Errorf("%w: definition is required", domain.ErrInvalidRequest)
	}
	id, err := id.New(id.Form)
	if err != nil {
		return domain.Form{}, err
	}
	now := time.Now().UTC()
	f := domain.Form{ID: id, SiteID: siteID, Name: name, Definition: definition, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateForm(ctx, f); err != nil {
		return domain.Form{}, err
	}
	return f, nil
}

func (s *Service) Get(ctx context.Context, siteID, id string) (domain.Form, error) {
	return s.repo.GetForm(ctx, siteID, id)
}

func (s *Service) List(ctx context.Context, siteID string) ([]domain.Form, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.repo.ListFormsBySite(ctx, siteID)
}

func (s *Service) Update(ctx context.Context, siteID, id string, name string, definition map[string]any) (domain.Form, error) {
	current, err := s.repo.GetForm(ctx, siteID, id)
	if err != nil {
		return domain.Form{}, err
	}
	if name != "" {
		current.Name = name
	}
	if definition != nil {
		current.Definition = definition
	}
	current.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateForm(ctx, current); err != nil {
		return domain.Form{}, err
	}
	return current, nil
}

func (s *Service) Delete(ctx context.Context, siteID, id string) error {
	if _, err := s.repo.GetForm(ctx, siteID, id); err != nil {
		return err
	}
	return s.repo.DeleteForm(ctx, siteID, id)
}

// Submit validates the raw payload against the form definition and persists it
// server-side (raw JSON, no polling state).
func (s *Service) Submit(ctx context.Context, siteID, formID string, payload map[string]any) (domain.Submission, error) {
	form, err := s.repo.GetForm(ctx, siteID, formID)
	if err != nil {
		return domain.Submission{}, err
	}
	values, ok := payload["values"].(map[string]any)
	if !ok {
		return domain.Submission{}, fmt.Errorf("%w: values must be an object", domain.ErrInvalidRequest)
	}
	if err := validateSubmission(form.Definition, values, s.settings.EmailPattern); err != nil {
		return domain.Submission{}, err
	}
	id, err := id.New(id.Submission)
	if err != nil {
		return domain.Submission{}, err
	}
	sub := domain.Submission{ID: id, SiteID: siteID, FormID: formID, Payload: payload, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateSubmission(ctx, sub); err != nil {
		return domain.Submission{}, err
	}
	return sub, nil
}

func (s *Service) ListSubmissions(ctx context.Context, siteID, formID string) ([]domain.Submission, error) {
	if _, err := s.repo.GetForm(ctx, siteID, formID); err != nil {
		return nil, err
	}
	return s.repo.ListSubmissionsByForm(ctx, siteID, formID)
}

func validateSubmission(definition map[string]any, values map[string]any, emailPattern *regexp.Regexp) error {
	fields, ok := definition["fields"].([]any)
	if !ok {
		return nil
	}
	for _, raw := range fields {
		field, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := field["name"].(string)
		if name == "" {
			continue
		}
		value, present := values[name]
		if required, _ := field["required"].(bool); required && (!present || isEmpty(value)) {
			return fmt.Errorf("%w: field %q is required", domain.ErrInvalidRequest, name)
		}
		if !present || isEmpty(value) {
			continue
		}
		str, _ := value.(string)
		if ftype, _ := field["type"].(string); ftype == "email" && emailPattern != nil && !emailPattern.MatchString(str) {
			return fmt.Errorf("%w: field %q must be a valid email", domain.ErrInvalidRequest, name)
		}
		if min, ok := field["minLength"].(float64); ok && int64(len(str)) < int64(min) {
			return fmt.Errorf("%w: field %q is too short", domain.ErrInvalidRequest, name)
		}
	}
	return nil
}

func isEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	}
	return false
}
