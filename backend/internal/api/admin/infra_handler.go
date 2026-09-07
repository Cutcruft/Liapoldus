package admin

import (
	"net/http"
	"time"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/infra"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// InfraHandler exposes admin CRUD for ui-runtime operation/endpoint
// descriptors (R6). System descriptors are listed for reference but reject
// write attempts (the infra service guards that).
type InfraHandler struct {
	svc *infra.Service
}

func NewInfraHandler(svc *infra.Service) *InfraHandler {
	return &InfraHandler{svc: svc}
}

type operationRequest struct {
	ID         string         `json:"id"`
	Provider   string         `json:"provider"`
	TypeOp     string         `json:"typeOp"`
	Method     string         `json:"method"`
	Path       string         `json:"path"`
	Cache      string         `json:"cache"`
	TTL        *int           `json:"ttl"`
	Scope      string         `json:"scope"`
	ResultType string         `json:"resultType"`
	Params     map[string]any `json:"params"`
	Poll       map[string]any `json:"poll"`
	Subscribe  map[string]any `json:"subscribe"`
}

func (req operationRequest) toDomain() domain.Operation {
	return domain.Operation{
		ID:         req.ID,
		Provider:   req.Provider,
		TypeOp:     req.TypeOp,
		Method:     req.Method,
		Path:       req.Path,
		Cache:      req.Cache,
		TTL:        req.TTL,
		Scope:      req.Scope,
		ResultType: req.ResultType,
		Params:     req.Params,
		Poll:       req.Poll,
		Subscribe:  req.Subscribe,
	}
}

type operationResponse struct {
	ID         string         `json:"id"`
	SiteID     string         `json:"siteId"`
	System     bool           `json:"system"`
	Provider   string         `json:"provider"`
	TypeOp     string         `json:"typeOp"`
	Method     string         `json:"method"`
	Path       string         `json:"path"`
	Cache      string         `json:"cache"`
	TTL        *int           `json:"ttl"`
	Scope      string         `json:"scope"`
	ResultType string         `json:"resultType"`
	Params     map[string]any `json:"params"`
	Poll       map[string]any `json:"poll"`
	Subscribe  map[string]any `json:"subscribe"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

func toOperationResponse(op domain.Operation) operationResponse {
	return operationResponse{
		ID:         op.ID,
		SiteID:     op.SiteID,
		System:     op.System,
		Provider:   op.Provider,
		TypeOp:     op.TypeOp,
		Method:     op.Method,
		Path:       op.Path,
		Cache:      op.Cache,
		TTL:        op.TTL,
		Scope:      op.Scope,
		ResultType: op.ResultType,
		Params:     op.Params,
		Poll:       op.Poll,
		Subscribe:  op.Subscribe,
		CreatedAt:  op.CreatedAt,
		UpdatedAt:  op.UpdatedAt,
	}
}

func (h *InfraHandler) CreateOperation(w http.ResponseWriter, r *http.Request) {
	var req operationRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	op, err := h.svc.CreateOperation(r.Context(), siteID(r), req.toDomain())
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, toOperationResponse(op))
}

func (h *InfraHandler) ListOperations(w http.ResponseWriter, r *http.Request) {
	ops, err := h.svc.ListOperations(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	resp := make([]operationResponse, 0, len(ops))
	for _, op := range ops {
		resp = append(resp, toOperationResponse(op))
	}
	httpapi.RespondJSON(w, http.StatusOK, resp)
}

func (h *InfraHandler) GetOperation(w http.ResponseWriter, r *http.Request) {
	op, err := h.svc.GetOperation(r.Context(), siteID(r), r.PathValue("operationID"))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, toOperationResponse(op))
}

func (h *InfraHandler) UpdateOperation(w http.ResponseWriter, r *http.Request) {
	var req operationRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	op := req.toDomain()
	op.ID = r.PathValue("operationID")
	op, err := h.svc.UpdateOperation(r.Context(), siteID(r), op)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, toOperationResponse(op))
}

func (h *InfraHandler) DeleteOperation(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteOperation(r.Context(), siteID(r), r.PathValue("operationID")); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type endpointRequest struct {
	ID          string `json:"id"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	OperationID string `json:"operationId"`
}

func (req endpointRequest) toDomain() domain.Endpoint {
	return domain.Endpoint{
		ID:          req.ID,
		Method:      req.Method,
		Path:        req.Path,
		OperationID: req.OperationID,
	}
}

type endpointResponse struct {
	ID          string    `json:"id"`
	SiteID      string    `json:"siteId"`
	System      bool      `json:"system"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	OperationID string    `json:"operationId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func toEndpointResponse(ep domain.Endpoint) endpointResponse {
	return endpointResponse{
		ID:          ep.ID,
		SiteID:      ep.SiteID,
		System:      ep.System,
		Method:      ep.Method,
		Path:        ep.Path,
		OperationID: ep.OperationID,
		CreatedAt:   ep.CreatedAt,
		UpdatedAt:   ep.UpdatedAt,
	}
}

func (h *InfraHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var req endpointRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	ep, err := h.svc.CreateEndpoint(r.Context(), siteID(r), req.toDomain())
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, toEndpointResponse(ep))
}

func (h *InfraHandler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	eps, err := h.svc.ListEndpoints(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	resp := make([]endpointResponse, 0, len(eps))
	for _, ep := range eps {
		resp = append(resp, toEndpointResponse(ep))
	}
	httpapi.RespondJSON(w, http.StatusOK, resp)
}

func (h *InfraHandler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	ep, err := h.svc.GetEndpoint(r.Context(), siteID(r), r.PathValue("endpointID"))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, toEndpointResponse(ep))
}

func (h *InfraHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	var req endpointRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	ep := req.toDomain()
	ep.ID = r.PathValue("endpointID")
	ep, err := h.svc.UpdateEndpoint(r.Context(), siteID(r), ep)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, toEndpointResponse(ep))
}

func (h *InfraHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteEndpoint(r.Context(), siteID(r), r.PathValue("endpointID")); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}