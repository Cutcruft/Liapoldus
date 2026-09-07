package runtime

import (
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Descriptor types mirroring the ui-runtime contract (docs/ui-runtime/spec.md
// §3, ui-runtime/src/types/descriptor.ts). JSON field names must match what
// parseDescriptors/extractTree expect, so a boot script can consume the
// contract verbatim.

// RouteAction mirrors ui-runtime RouteAction (renderPage/serveAsset/redirect).
type RouteAction struct {
	Type      string `json:"type"`
	PageID    string `json:"pageId,omitempty"`
	AssetID   string `json:"assetId,omitempty"`
	Target    string `json:"target,omitempty"`
	Status    int    `json:"status,omitempty"`
	KeepQuery bool   `json:"keepQuery,omitempty"`
}

// RouteDescriptor mirrors ui-runtime RouteDescriptor (id, name, full ^…$ matcher,
// priority, action).
type RouteDescriptor struct {
	ID       string      `json:"id"`
	Name     string      `json:"name,omitempty"`
	Matcher  string      `json:"matcher"`
	Priority int         `json:"priority"`
	Action   RouteAction `json:"action"`
}

// ThemeDescriptor mirrors ui-runtime ThemeDescriptor (empty today; design
// tokens arrive with a later stage).
type ThemeDescriptor struct {
	ThemeID string         `json:"themeId"`
	Tokens  map[string]any `json:"tokens"`
	Fonts   []string       `json:"fonts,omitempty"`
	Assets  []string       `json:"assets,omitempty"`
}

// EnabledChannels mirrors ui-runtime EnabledChannels.
type EnabledChannels struct {
	WS  bool `json:"ws"`
	SSE bool `json:"sse"`
}

// Capabilities mirrors ui-runtime Capabilities (formSubmissions, dev).
type Capabilities struct {
	FormSubmissions bool `json:"formSubmissions"`
	Dev             bool `json:"dev"`
}

// ElementDescriptor — wire-форма элемента страницы в контракте (§1.3).
// props/bindings/values always present (non-null) so the ui-runtime crawler can
// iterate them safely.
type ElementDescriptor struct {
	ID          string                           `json:"id"`
	ComponentID string                           `json:"componentId"`
	Props       map[string]ElementPropDescriptor `json:"props"`
	Bindings    []domain.BindingSource           `json:"bindings"`
}

// ElementPropDescriptor is a literal or binding wire property value.
type ElementPropDescriptor struct {
	Kind   string                `json:"kind"` // "literal" | "binding"
	Value  any                   `json:"value,omitempty"`
	Source *domain.BindingSource `json:"source,omitempty"`
}

// PageDescriptor mirrors the runtime page descriptor: the linear element list
// that renders in order. Routes reference pages by pageId; the boot contract
// carries the assembled page (array of components) per §1.3.
//
// R10 P1: LayoutSectionID/Head are the RAW page-level overrides from the pinned
// page version. The client merges them over the site-wide defaults
// (Contract.Head / Contract.DefaultLayoutSectionID), so navigation can resolve
// the effective layout and document head without a round trip.
type PageDescriptor struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Elements        []ElementDescriptor `json:"elements"`
	LayoutSectionID string              `json:"layoutSectionId,omitempty"`
	Head            *PageHead           `json:"head,omitempty"`
}

// PageHead mirrors the runtime per-page head override (docs/ui-runtime/spec.md
// §2.4). Scalar fields override the site defaults; OG/Meta merge by key.
type PageHead struct {
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Robots      string            `json:"robots,omitempty"`
	Canonical   string            `json:"canonical,omitempty"`
	OG          map[string]string `json:"og,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
}

// TreeDeclaration is removed in R7a — pages are flat element lists
// (PageDescriptor), not nested trees.

// OperationDescriptor mirrors ui-runtime OperationDescriptor (descriptor.ts):
// a managed operation the runtime can invoke query- or mutation-style.
type OperationDescriptor struct {
	Kind       string         `json:"kind"`
	ID         string         `json:"id"`
	TypeOp     string         `json:"typeOp"`
	ProviderID string         `json:"providerId"`
	Method     string         `json:"method"`
	Path       string         `json:"path"`
	Params     map[string]any `json:"params,omitempty"`
	Type       string         `json:"type,omitempty"`
	Cache      string         `json:"cache"`
	TTL        *int           `json:"ttl,omitempty"`
	Scope      string         `json:"scope,omitempty"`
	Poll       map[string]any `json:"poll,omitempty"`
	Subscribe  map[string]any `json:"subscribe,omitempty"`
}

// EndpointDescriptor mirrors ui-runtime EndpointDescriptor (descriptor.ts): a
// named path/method pair aliasing an operation.
type EndpointDescriptor struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	Path        string `json:"path"`
	Method      string `json:"method"`
	OperationID string `json:"operationId"`
}

// FallbackDescriptor mirrors ui-runtime FallbackDescriptor (unused today).
type FallbackDescriptor struct {
	ID         string            `json:"id"`
	Definition string            `json:"definition,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

// Contract is the snapshot-pinned boot contract served at GET /runtime/contract.
//
// R10 P1: Head/DefaultLayoutSectionID are site-wide presentation defaults that
// pages inherit unless they override them (see PageDescriptor.LayoutSectionID /
// PageDescriptor.Head). The client applies them client-side to the live page.
type Contract struct {
	SiteID                 string                `json:"siteId"`
	Environment            string                `json:"environment"`
	Version                string                `json:"version"`
	Locale                 string                `json:"locale"`
	Providers              []any                 `json:"providers"`
	Operations             []OperationDescriptor `json:"operations"`
	Endpoints              []EndpointDescriptor  `json:"endpoints"`
	Routes                 []RouteDescriptor     `json:"routes"`
	Themes                 []ThemeDescriptor     `json:"themes"`
	Fallback               *FallbackDescriptor   `json:"fallback,omitempty"`
	EnabledChannels        EnabledChannels       `json:"enabledChannels"`
	Capabilities           Capabilities          `json:"capabilities"`
	Head                   domain.SiteHead       `json:"head,omitempty"`
	DefaultLayoutSectionID string                `json:"defaultLayoutSectionId,omitempty"`
	Pages                  []PageDescriptor      `json:"pages,omitempty"`
}
