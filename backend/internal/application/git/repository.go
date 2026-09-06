// Package git defines the Repository port (application boundary) used by
// component versioning (R5/R6/R7 in docs/design/frontend.md; §4/§5 of
// docs/backend/components-test-spec.md). The go-git implementation lives in
// internal/infra/git; this package contains only the interface and the
// application-level Service built on it.
package git

import "context"

// Repository exposes the operations a git-сервис needs. Each method keys on
// siteID ("one repo per site"); commit sha values are the ComponentVersion ids.
type Repository interface {
	// Init ensures a bare repository exists for the site. Idempotent.
	Init(ctx context.Context, siteID string) error

	// Commit writes files and returns the commit sha. definitionID scopes the
	// commit so versions can be listed per component.
	Commit(ctx context.Context, siteID, definitionID, message string, files map[string][]byte) (string, error)

	// ListVersions returns commit shas for one definition in chronological order.
	ListVersions(ctx context.Context, siteID, definitionID string) ([]string, error)

	// Checkout returns the file set of a single commit.
	Checkout(ctx context.Context, siteID, sha string) (map[string][]byte, error)

	// Head returns the latest commit sha of the site repository.
	Head(ctx context.Context, siteID string) (string, error)
}
