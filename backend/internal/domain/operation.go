package domain

import "time"

// Operation is a managed ui-runtime operation descriptor (R6). System rows
// (SiteID == "") mirror the platform builtins and are read-only through the
// admin API; site rows are admin-editable.
type Operation struct {
	ID         string
	SiteID     string
	System     bool
	Provider   string // providerId
	TypeOp     string // query | mutation
	Method     string // GET | POST | PUT | PATCH | DELETE
	Path       string
	Cache      string // immutable | disabled | ttl
	TTL        *int
	Scope      string // public | server
	ResultType string // op.type binding (content, content[], …)
	Params     map[string]any
	Poll       map[string]any
	Subscribe  map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Endpoint is a managed ui-runtime endpoint descriptor (R6). Like Operation,
// a system row has SiteID == "" and is read-only through the admin API.
type Endpoint struct {
	ID          string
	SiteID      string
	System      bool
	Method      string
	Path        string
	OperationID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}