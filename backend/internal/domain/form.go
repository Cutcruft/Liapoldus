package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Form struct {
	ID         string         `json:"id"`
	SiteID     string         `json:"siteId"`
	Name       string         `json:"name"`
	Definition map[string]any `json:"definition"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

type Submission struct {
	ID        string         `json:"id"`
	SiteID    string         `json:"siteId"`
	FormID    string         `json:"formId"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"createdAt"`
}

type FormService struct {
	repo  FormRepository
	sites SiteRepository
}

func NewFormService(repo FormRepository, sites SiteRepository) *FormService {
	return &FormService{repo: repo, sites: sites}
}

func (s *FormService) Create(ctx context.Context, siteID, name string, definition map[string]any) (Form, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return Form{}, err
	}
	if name == "" {
		return Form{}, fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	if definition == nil {
		return Form{}, fmt.Errorf("%w: definition is required", ErrInvalidRequest)
	}
	id, err := NewID("form")
	if err != nil {
		return Form{}, err
	}
	now := time.Now().UTC()
	f := Form{ID: id, SiteID: siteID, Name: name, Definition: definition, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateForm(ctx, f); err != nil {
		return Form{}, err
	}
	return f, nil
}

func (s *FormService) Get(ctx context.Context, siteID, id string) (Form, error) {
	return s.repo.GetForm(ctx, siteID, id)
}

func (s *FormService) List(ctx context.Context, siteID string) ([]Form, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.repo.ListFormsBySite(ctx, siteID)
}

func (s *FormService) Update(ctx context.Context, siteID, id string, name string, definition map[string]any) (Form, error) {
	current, err := s.repo.GetForm(ctx, siteID, id)
	if err != nil {
		return Form{}, err
	}
	if name != "" {
		current.Name = name
	}
	if definition != nil {
		current.Definition = definition
	}
	current.UpdatedAt = time.Now().UTC()
	if err := s.repo.UpdateForm(ctx, current); err != nil {
		return Form{}, err
	}
	return current, nil
}

func (s *FormService) Delete(ctx context.Context, siteID, id string) error {
	if _, err := s.repo.GetForm(ctx, siteID, id); err != nil {
		return err
	}
	return s.repo.DeleteForm(ctx, siteID, id)
}

// Submit validates the raw payload against the form definition and persists it
// server-side (raw JSON, no polling state).
func (s *FormService) Submit(ctx context.Context, siteID, formID string, payload map[string]any) (Submission, error) {
	form, err := s.repo.GetForm(ctx, siteID, formID)
	if err != nil {
		return Submission{}, err
	}
	values, ok := payload["values"].(map[string]any)
	if !ok {
		return Submission{}, fmt.Errorf("%w: values must be an object", ErrInvalidRequest)
	}
	if err := validateSubmission(form.Definition, values); err != nil {
		return Submission{}, err
	}
	id, err := NewID("submission")
	if err != nil {
		return Submission{}, err
	}
	sub := Submission{ID: id, SiteID: siteID, FormID: formID, Payload: payload, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateSubmission(ctx, sub); err != nil {
		return Submission{}, err
	}
	return sub, nil
}

func (s *FormService) ListSubmissions(ctx context.Context, siteID, formID string) ([]Submission, error) {
	if _, err := s.repo.GetForm(ctx, siteID, formID); err != nil {
		return nil, err
	}
	return s.repo.ListSubmissionsByForm(ctx, siteID, formID)
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func validateSubmission(definition map[string]any, values map[string]any) error {
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
			return fmt.Errorf("%w: field %q is required", ErrInvalidRequest, name)
		}
		if !present || isEmpty(value) {
			continue
		}
		str, _ := value.(string)
		if ftype, _ := field["type"].(string); ftype == "email" && !emailPattern.MatchString(str) {
			return fmt.Errorf("%w: field %q must be a valid email", ErrInvalidRequest, name)
		}
		if min, ok := field["minLength"].(float64); ok && int64(len(str)) < int64(min) {
			return fmt.Errorf("%w: field %q is too short", ErrInvalidRequest, name)
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
