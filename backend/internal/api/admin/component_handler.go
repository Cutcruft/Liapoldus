package admin

import (
	"context"
	"net/http"
	"strconv"
	"time"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/component"
	"github.com/liapoldus/liapoldus/backend/internal/application/gitsnapshot"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type ComponentHandler struct {
	components *component.Service
	pages      *page.Service
	git        *gitsnapshot.Service
}

func NewComponentHandler(components *component.Service, pages *page.Service, git *gitsnapshot.Service) *ComponentHandler {
	return &ComponentHandler{components: components, pages: pages, git: git}
}

// componentRequest mirrors ComponentDefinition but keeps Source explicit (the
// domain field is json:"-" and must not leak into other payloads).
type componentRequest struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	Source   string         `json:"source"`
	Schema   map[string]any `json:"schema"`
	Metadata map[string]any `json:"metadata"`
}

func (req componentRequest) toDomain(siteID string) domain.ComponentDefinition {
	def := domain.ComponentDefinition{
		SiteID:   siteID,
		ID:       req.ID,
		Name:     req.Name,
		Kind:     req.Kind,
		Source:   req.Source,
		Schema:   req.Schema,
		Metadata: req.Metadata,
	}
	if def.Kind == "" {
		def.Kind = "component"
	}
	return def
}

func (h *ComponentHandler) Define(w http.ResponseWriter, r *http.Request) {
	var req componentRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.components.Define(r.Context(), req.toDomain(siteID(r)))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, result)
}

// editorComponent is one entry of the editor catalog returned by
// GET /api/sites/{id}/components: builtin platform components plus the site's
// defined components, each shaped for the editor palette/inspector.
type editorComponent struct {
	Type      string         `json:"type"`
	Label     string         `json:"label"`
	Container bool           `json:"container"`
	Schema    map[string]any `json:"schema"`
}

// List returns the unified editor catalog for a site: platform builtin
// components first, then the site's component definitions mapped to the same
// shape. The admin editor loads this once when it mounts (schemas.ts).
func (h *ComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	defs, err := h.components.List(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	builtins := component.Builtins()
	catalog := make([]editorComponent, 0, len(builtins)+len(defs))
	for _, b := range builtins {
		catalog = append(catalog, editorComponent{Type: b.Type, Label: b.Label, Container: b.Container, Schema: b.Schema})
	}
	for _, d := range defs {
		label := d.Name
		if l, ok := stringMeta(d.Metadata, "label"); ok && l != "" {
			label = l
		}
		catalog = append(catalog, editorComponent{Type: d.ID, Label: label, Container: false, Schema: d.Schema})
	}
	httpapi.RespondJSON(w, http.StatusOK, catalog)
}

func stringMeta(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key].(string)
	return v, ok
}

// componentResponse is the wire shape of a component: the domain definition
// plus its source (kept out of ComponentDefinition JSON) and the committed
// flag computed against the dev HEAD.
type componentResponse struct {
	ID         string         `json:"id"`
	SiteID     string         `json:"siteId"`
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	Source     string         `json:"source"`
	Schema     map[string]any `json:"schema"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CurrentSHA string         `json:"currentSha"`
	Committed  bool           `json:"committed"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

func toComponentResponse(d *domain.ComponentDefinition) componentResponse {
	return componentResponse{
		ID:         d.ID,
		SiteID:     d.SiteID,
		Name:       d.Name,
		Kind:       d.Kind,
		Source:     d.Source,
		Schema:     d.Schema,
		Metadata:   d.Metadata,
		CurrentSHA: d.CurrentSHA,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

// Get returns a component's registry entry plus its source. A ?sha=N query
// swaps the body for the historical version captured by that commit
// (source/schema/metadata from the tree); name/kind/timestamps come from the
// current registry when the component still exists.
func (h *ComponentHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	siteID := siteID(r)
	componentID := r.PathValue("componentID")

	sha := r.URL.Query().Get("sha")
	if sha != "" {
		if h.git == nil {
			httpapi.RespondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "git history unavailable"})
			return
		}
		def, err := h.git.ReadComponentAt(ctx, siteID, componentID, sha)
		if err != nil {
			httpapi.RespondError(w, err)
			return
		}
		if current, err := h.components.Get(ctx, siteID, componentID); err == nil {
			def.Name = current.Name
			def.Kind = current.Kind
			def.CreatedAt = current.CreatedAt
			def.UpdatedAt = current.UpdatedAt
		}
		resp := toComponentResponse(def)
		resp.CurrentSHA = sha
		httpapi.RespondJSON(w, http.StatusOK, resp)
		return
	}

	result, err := h.components.Get(ctx, siteID, componentID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	resp := toComponentResponse(result)
	resp.Committed = h.committed(ctx, result)
	httpapi.RespondJSON(w, http.StatusOK, resp)
}

// committed reports whether the component's current source is captured by the
// latest dev commit (i.e. a Git-коммит would not change it). False when there
// is no repository or the source differs from / is missing in the dev HEAD.
func (h *ComponentHandler) committed(ctx context.Context, def *domain.ComponentDefinition) bool {
	if h.git == nil {
		return false
	}
	devSHA, err := h.git.DevHeadSHA(ctx, def.SiteID)
	if err != nil {
		return false
	}
	source, err := h.git.SourceAt(ctx, def.SiteID, def.ID, devSHA)
	if err != nil {
		return false
	}
	return source == def.Source
}

func (h *ComponentHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req componentRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	def := req.toDomain(siteID(r))
	def.ID = r.PathValue("componentID")
	result, err := h.components.Update(r.Context(), def)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *ComponentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.components.Delete(r.Context(), siteID(r), r.PathValue("componentID")); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// registryComponent is one row of the R5 registry panel.
type registryComponent struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	CurrentSHA string    `json:"currentSha,omitempty"`
	Committed  bool      `json:"committed"`
	UsageCount int       `json:"usageCount"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// registryResponse is the payload of GET /api/sites/{id}/components/registry.
type registryResponse struct {
	DevSHA     string              `json:"devSha,omitempty"`
	Components []registryComponent `json:"components"`
}

// Registry lists the site's component definitions with their commit and usage
// status for the editor's left panel.
func (h *ComponentHandler) Registry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	siteID := siteID(r)
	defs, err := h.components.List(ctx, siteID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	usage, err := h.usageCounts(ctx, siteID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	resp := registryResponse{Components: make([]registryComponent, 0, len(defs))}
	for i := range defs {
		d := &defs[i]
		row := registryComponent{
			ID:         d.ID,
			Name:       d.Name,
			Kind:       d.Kind,
			CurrentSHA: d.CurrentSHA,
			Committed:  h.committed(ctx, d),
			UsageCount: usage[d.ID],
			UpdatedAt:  d.UpdatedAt,
		}
		resp.Components = append(resp.Components, row)
	}
	if h.git != nil {
		if devSHA, err := h.git.DevHeadSHA(ctx, siteID); err == nil {
			resp.DevSHA = devSHA
		}
	}
	httpapi.RespondJSON(w, http.StatusOK, resp)
}

// History returns the dev commits where a component's source changed (newest
// first), each with the source at that commit inline.
func (h *ComponentHandler) History(w http.ResponseWriter, r *http.Request) {
	if h.git == nil {
		httpapi.RespondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "git history unavailable"})
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	history, err := h.git.ComponentHistory(r.Context(), siteID(r), r.PathValue("componentID"), limit)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, history)
}

// pageUsageItem is one page referencing a component.
type pageUsageItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// componentUsage is the payload of GET /api/sites/{id}/components/{id}/usage.
type componentUsage struct {
	ComponentID string          `json:"componentId"`
	Pages       []pageUsageItem `json:"pages"`
}

// Usage lists the pages whose trees reference a component definition.
func (h *ComponentHandler) Usage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	siteID := siteID(r)
	componentID := r.PathValue("componentID")
	if _, err := h.components.Get(ctx, siteID, componentID); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	if h.pages == nil {
		httpapi.RespondJSON(w, http.StatusOK, componentUsage{ComponentID: componentID, Pages: []pageUsageItem{}})
		return
	}
	pages, err := h.pages.ListBySite(ctx, siteID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	usage := componentUsage{ComponentID: componentID, Pages: []pageUsageItem{}}
	for _, p := range pages {
		if count := countElement(p.List, componentID); count > 0 {
			usage.Pages = append(usage.Pages, pageUsageItem{ID: p.ID, Name: p.Name, Count: count})
		}
	}
	httpapi.RespondJSON(w, http.StatusOK, usage)
}

// usageCounts tallies every definition's references across all page element
// lists.
func (h *ComponentHandler) usageCounts(ctx context.Context, siteID string) (map[string]int, error) {
	counts := map[string]int{}
	if h.pages == nil {
		return counts, nil
	}
	pages, err := h.pages.ListBySite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	for _, p := range pages {
		for _, el := range p.List {
			if el.ComponentID != "" {
				counts[el.ComponentID]++
			}
		}
	}
	return counts, nil
}

func countElement(list []domain.Element, componentID string) int {
	count := 0
	for _, el := range list {
		if el.ComponentID == componentID {
			count++
		}
	}
	return count
}

// commitResponse reports the git sha produced by a component commit and how
// many registry definitions were marked against it.
type commitResponse struct {
	SHA   string `json:"sha"`
	Count int    `json:"count"`
}

// Commit saves the current site state (including all component sources) as one
// dev commit and marks every registered definition committed at that sha. Like
// the Git-страница commit, this is a full-site snapshot; the response tells
// the editor how many definitions the commit covered.
func (h *ComponentHandler) Commit(w http.ResponseWriter, r *http.Request) {
	if h.git == nil {
		httpapi.RespondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "git commit unavailable"})
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	ctx := r.Context()
	siteID := siteID(r)
	sha, err := h.git.Commit(ctx, siteID, req.Message)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	defs, err := h.components.List(ctx, siteID)
	if err != nil {
		httpapi.RespondJSON(w, http.StatusOK, commitResponse{SHA: sha, Count: 0})
		return
	}
	count := 0
	for _, d := range defs {
		if err := h.components.MarkCommitted(ctx, siteID, d.ID, sha); err != nil {
			continue
		}
		count++
	}
	httpapi.RespondJSON(w, http.StatusOK, commitResponse{SHA: sha, Count: count})
}