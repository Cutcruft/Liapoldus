package domain

import "time"

// RouteActionType identifies the behavior applied when a route matches.
type RouteActionType string

// RouteAction mirrors the ui-runtime RouteDescriptor action (docs/ui-runtime/json-descriptors.md §4).
type RouteAction struct {
	Type      RouteActionType `json:"type"`
	PageID    string          `json:"pageId,omitempty"`
	AssetID   string          `json:"assetId,omitempty"`
	Target    string          `json:"target,omitempty"`
	Status    int             `json:"status,omitempty"`
	KeepQuery bool            `json:"keepQuery,omitempty"`
}

type Route struct {
	ID        string      `json:"id"`
	SiteID    string      `json:"siteId"`
	Matcher   string      `json:"matcher"`
	Priority  int         `json:"priority"`
	Action    RouteAction `json:"action"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}
