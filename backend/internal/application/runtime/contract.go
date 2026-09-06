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

// RouteDescriptor mirrors ui-runtime RouteDescriptor (id, full ^…$ matcher,
// priority, action).
type RouteDescriptor struct {
	ID       string      `json:"id"`
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

// TreeNode — wire-форма дерева контракта. props/bindings/children всегда
// присутствуют (не omitempty): resolveInstance() в ui-runtime итерирует
// bindings/children напрямую и падает на undefined.
type TreeNode struct {
	InstanceID   string                    `json:"instanceId"`
	DefinitionID string                    `json:"definitionId"`
	Props        map[string]any            `json:"props"`
	Bindings     []domain.ComponentBinding `json:"bindings"`
	Children     []TreeNode                `json:"children"`
}

// TreeDeclaration mirrors ui-runtime TreeDeclaration; Root is the page
// composition (wire tree, см. TreeNode).
type TreeDeclaration struct {
	SnapshotID string    `json:"snapshotId,omitempty"`
	VersionID  string    `json:"versionId,omitempty"`
	Root       *TreeNode `json:"root"`
}

// FallbackDescriptor mirrors ui-runtime FallbackDescriptor (unused today).
type FallbackDescriptor struct {
	ID         string            `json:"id"`
	Definition string            `json:"definition,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

// Contract is the snapshot-pinned boot contract served at GET /runtime/contract.
type Contract struct {
	SiteID          string              `json:"siteId"`
	Environment     string              `json:"environment"`
	Version         string              `json:"version"`
	Locale          string              `json:"locale"`
	Providers       []any               `json:"providers"`
	Operations      []any               `json:"operations"`
	Endpoints       []any               `json:"endpoints"`
	Routes          []RouteDescriptor   `json:"routes"`
	Themes          []ThemeDescriptor   `json:"themes"`
	Fallback        *FallbackDescriptor `json:"fallback,omitempty"`
	EnabledChannels EnabledChannels     `json:"enabledChannels"`
	Capabilities    Capabilities        `json:"capabilities"`
	Tree            *TreeDeclaration    `json:"tree,omitempty"`
}
