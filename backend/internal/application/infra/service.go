// Package infra manages the platform's operation/endpoint descriptors
// (ui-runtime contract, R6). System descriptors are seeded by migration 010 and
// are read-only through the admin API; site descriptors are fully editable.
// The service validates descriptor fields and cross-references (endpoint →
// operation) before any write.
package infra

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

var idRe = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)

var (
	validMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	validTypeOps = map[string]bool{"query": true, "mutation": true}
	validCaches  = map[string]bool{"immutable": true, "disabled": true, "ttl": true}
	validScopes  = map[string]bool{"public": true, "server": true}
)

const defaultProvider = "liapoldus.builtin"

type Service struct {
	ops  domain.OperationRepository
	eps  domain.EndpointRepository
	now  func() time.Time
}

func NewService(ops domain.OperationRepository, eps domain.EndpointRepository) *Service {
	return &Service{ops: ops, eps: eps, now: time.Now}
}

// --- Operations ---

func (s *Service) CreateOperation(ctx context.Context, siteID string, op domain.Operation) (domain.Operation, error) {
	op.SiteID = siteID
	op.System = false
	if err := validateOperation(&op); err != nil {
		return domain.Operation{}, err
	}
	if _, err := s.ops.GetOperation(ctx, siteID, op.ID); err == nil {
		return domain.Operation{}, fmt.Errorf("%w: operation %q", domain.ErrAlreadyExists, op.ID)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.Operation{}, err
	}
	now := s.now()
	op.CreatedAt, op.UpdatedAt = now, now
	if err := s.ops.CreateOperation(ctx, op); err != nil {
		return domain.Operation{}, err
	}
	return op, nil
}

func (s *Service) GetOperation(ctx context.Context, siteID, id string) (domain.Operation, error) {
	return s.ops.GetOperation(ctx, siteID, id)
}

func (s *Service) ListOperations(ctx context.Context, siteID string) ([]domain.Operation, error) {
	return s.ops.ListOperationsBySite(ctx, siteID)
}

func (s *Service) UpdateOperation(ctx context.Context, siteID string, op domain.Operation) (domain.Operation, error) {
	if err := validateOperation(&op); err != nil {
		return domain.Operation{}, err
	}
	prior, err := s.ops.GetOperation(ctx, siteID, op.ID)
	if err != nil {
		return domain.Operation{}, err
	}
	if prior.System {
		return domain.Operation{}, fmt.Errorf("%w: system descriptors are read-only", domain.ErrInvalidRequest)
	}
	op.SiteID = siteID
	op.System = false
	op.CreatedAt = prior.CreatedAt
	op.UpdatedAt = s.now()
	if op.Provider == "" {
		op.Provider = prior.Provider
	}
	if err := s.ops.UpdateOperation(ctx, op); err != nil {
		return domain.Operation{}, err
	}
	return op, nil
}

func (s *Service) DeleteOperation(ctx context.Context, siteID, id string) error {
	op, err := s.ops.GetOperation(ctx, siteID, id)
	if err != nil {
		return err
	}
	if op.System {
		return fmt.Errorf("%w: system descriptors are read-only", domain.ErrInvalidRequest)
	}
	return s.ops.DeleteOperation(ctx, siteID, id)
}

func validateOperation(op *domain.Operation) error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("%w: operation id is required", domain.ErrInvalidRequest)
	}
	if !idRe.MatchString(op.ID) {
		return fmt.Errorf("%w: operation id must match %q", domain.ErrInvalidRequest, idRe.String())
	}
	if op.Provider == "" {
		op.Provider = defaultProvider
	}
	if op.TypeOp == "" {
		op.TypeOp = "query"
	}
	if !validTypeOps[op.TypeOp] {
		return fmt.Errorf("%w: typeOp must be query or mutation", domain.ErrInvalidRequest)
	}
	if !validMethods[op.Method] {
		return fmt.Errorf("%w: method must be GET, POST, PUT, PATCH or DELETE", domain.ErrInvalidRequest)
	}
	if strings.TrimSpace(op.Path) == "" || !strings.HasPrefix(op.Path, "/") {
		return fmt.Errorf("%w: operation path must start with /", domain.ErrInvalidRequest)
	}
	if op.Cache == "" {
		op.Cache = "disabled"
	}
	if !validCaches[op.Cache] {
		return fmt.Errorf("%w: cache must be immutable, disabled or ttl", domain.ErrInvalidRequest)
	}
	if op.Cache == "ttl" && (op.TTL == nil || *op.TTL <= 0) {
		return fmt.Errorf("%w: ttl cache requires a positive ttl", domain.ErrInvalidRequest)
	}
	if op.Scope == "" {
		op.Scope = "public"
	}
	if !validScopes[op.Scope] {
		return fmt.Errorf("%w: scope must be public or server", domain.ErrInvalidRequest)
	}
	if op.Params == nil {
		op.Params = map[string]any{}
	}
	if op.Poll == nil {
		op.Poll = map[string]any{}
	}
	if op.Subscribe == nil {
		op.Subscribe = map[string]any{}
	}
	return nil
}

// --- Endpoints ---

func (s *Service) CreateEndpoint(ctx context.Context, siteID string, ep domain.Endpoint) (domain.Endpoint, error) {
	ep.SiteID = siteID
	ep.System = false
	if err := s.validateEndpoint(ctx, siteID, &ep); err != nil {
		return domain.Endpoint{}, err
	}
	if _, err := s.eps.GetEndpoint(ctx, siteID, ep.ID); err == nil {
		return domain.Endpoint{}, fmt.Errorf("%w: endpoint %q", domain.ErrAlreadyExists, ep.ID)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.Endpoint{}, err
	}
	now := s.now()
	ep.CreatedAt, ep.UpdatedAt = now, now
	if err := s.eps.CreateEndpoint(ctx, ep); err != nil {
		return domain.Endpoint{}, err
	}
	return ep, nil
}

func (s *Service) GetEndpoint(ctx context.Context, siteID, id string) (domain.Endpoint, error) {
	return s.eps.GetEndpoint(ctx, siteID, id)
}

func (s *Service) ListEndpoints(ctx context.Context, siteID string) ([]domain.Endpoint, error) {
	return s.eps.ListEndpointsBySite(ctx, siteID)
}

func (s *Service) UpdateEndpoint(ctx context.Context, siteID string, ep domain.Endpoint) (domain.Endpoint, error) {
	if err := s.validateEndpoint(ctx, siteID, &ep); err != nil {
		return domain.Endpoint{}, err
	}
	prior, err := s.eps.GetEndpoint(ctx, siteID, ep.ID)
	if err != nil {
		return domain.Endpoint{}, err
	}
	if prior.System {
		return domain.Endpoint{}, fmt.Errorf("%w: system descriptors are read-only", domain.ErrInvalidRequest)
	}
	ep.SiteID = siteID
	ep.System = false
	ep.CreatedAt = prior.CreatedAt
	ep.UpdatedAt = s.now()
	if err := s.eps.UpdateEndpoint(ctx, ep); err != nil {
		return domain.Endpoint{}, err
	}
	return ep, nil
}

func (s *Service) DeleteEndpoint(ctx context.Context, siteID, id string) error {
	ep, err := s.eps.GetEndpoint(ctx, siteID, id)
	if err != nil {
		return err
	}
	if ep.System {
		return fmt.Errorf("%w: system descriptors are read-only", domain.ErrInvalidRequest)
	}
	return s.eps.DeleteEndpoint(ctx, siteID, id)
}

func (s *Service) validateEndpoint(ctx context.Context, siteID string, ep *domain.Endpoint) error {
	if strings.TrimSpace(ep.ID) == "" {
		return fmt.Errorf("%w: endpoint id is required", domain.ErrInvalidRequest)
	}
	if !idRe.MatchString(ep.ID) {
		return fmt.Errorf("%w: endpoint id must match %q", domain.ErrInvalidRequest, idRe.String())
	}
	if !validMethods[ep.Method] {
		return fmt.Errorf("%w: method must be GET, POST, PUT, PATCH or DELETE", domain.ErrInvalidRequest)
	}
	if strings.TrimSpace(ep.Path) == "" || !strings.HasPrefix(ep.Path, "/") {
		return fmt.Errorf("%w: endpoint path must start with /", domain.ErrInvalidRequest)
	}
	if strings.TrimSpace(ep.OperationID) == "" {
		return fmt.Errorf("%w: endpoint operationId is required", domain.ErrInvalidRequest)
	}
	if _, err := s.ops.GetOperation(ctx, siteID, ep.OperationID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("%w: operation %q does not exist", domain.ErrInvalidRequest, ep.OperationID)
		}
		return err
	}
	return nil
}

// ValidateSubmitTarget checks a form definition's submit target against the
// descriptor registry. Accepted forms (ui-runtime types/form.ts): explicit
// "endpoint.<id>" or "operation.<id>", plus a bare id which defaults to an
// endpoint.
func (s *Service) ValidateSubmitTarget(ctx context.Context, siteID, target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("%w: submit target is required", domain.ErrInvalidRequest)
	}
	kind, id := "endpoint", target
	switch {
	case strings.HasPrefix(target, "endpoint."):
		kind, id = "endpoint", strings.TrimPrefix(target, "endpoint.")
	case strings.HasPrefix(target, "operation."):
		kind, id = "operation", strings.TrimPrefix(target, "operation.")
	default:
		if strings.Contains(target, ".") && !validId(target) {
			return fmt.Errorf("%w: submit target must be endpoint.<id>, operation.<id> or a bare id", domain.ErrInvalidRequest)
		}
	}
	if id == "" {
		return fmt.Errorf("%w: submit target is required", domain.ErrInvalidRequest)
	}
	switch kind {
	case "operation":
		if _, err := s.ops.GetOperation(ctx, siteID, id); err != nil {
			return fmt.Errorf("%w: submit target operation %q does not exist", domain.ErrInvalidRequest, id)
		}
	default:
		if _, err := s.eps.GetEndpoint(ctx, siteID, id); err != nil {
			return fmt.Errorf("%w: submit target endpoint %q does not exist", domain.ErrInvalidRequest, id)
		}
	}
	return nil
}

func validId(s string) bool { return idRe.MatchString(s) }