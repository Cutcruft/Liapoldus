package unit

import (
	"context"
	"errors"
	"testing"

	formapp "github.com/liapoldus/liapoldus/backend/internal/application/form"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/tests/unit/mocks"
	"go.uber.org/mock/gomock"
)

func newFormService(repo domain.FormRepository, sites domain.SiteRepository) *formapp.Service {
	return formapp.NewService(repo, sites, formapp.Settings{EmailPattern: emailPattern})
}

func TestFormServiceCreateRequiresDefinition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sites := mocks.NewMockSiteRepository(ctrl)
	sites.EXPECT().GetSite(gomock.Any(), "site_1").Return(domain.Site{ID: "site_1"}, nil)

	repo := mocks.NewMockFormRepository(ctrl)

	_, err := newFormService(repo, sites).Create(context.Background(), "site_1", "contact", nil)
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestFormServiceSubmitValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	definition := map[string]any{
		"fields": []any{
			map[string]any{"name": "email", "type": "email", "required": true},
		},
	}
	repo := mocks.NewMockFormRepository(ctrl)
	repo.EXPECT().GetForm(gomock.Any(), "site_1", "form_1").Return(domain.Form{ID: "form_1", SiteID: "site_1", Definition: definition}, nil)

	service := newFormService(repo, nil)

	_, err := service.Submit(context.Background(), "site_1", "form_1", map[string]any{"values": map[string]any{"email": "not-an-email"}})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestFormServiceSubmitValid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	definition := map[string]any{
		"fields": []any{
			map[string]any{"name": "email", "type": "email", "required": true},
		},
	}
	repo := mocks.NewMockFormRepository(ctrl)
	repo.EXPECT().GetForm(gomock.Any(), "site_1", "form_1").Return(domain.Form{ID: "form_1", SiteID: "site_1", Definition: definition}, nil)
	repo.EXPECT().CreateSubmission(gomock.Any(), gomock.Any()).Return(nil)

	sub, err := newFormService(repo, nil).Submit(context.Background(), "site_1", "form_1", map[string]any{
		"values": map[string]any{"email": "user@example.com"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if sub.FormID != "form_1" || sub.Payload["values"] == nil {
		t.Fatalf("submission = %#v", sub)
	}
}

func TestFormServiceSubmitRequiresValuesObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockFormRepository(ctrl)
	repo.EXPECT().GetForm(gomock.Any(), "site_1", "form_1").Return(domain.Form{ID: "form_1", SiteID: "site_1", Definition: map[string]any{}}, nil)

	_, err := newFormService(repo, nil).Submit(context.Background(), "site_1", "form_1", map[string]any{"values": "oops"})
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}
