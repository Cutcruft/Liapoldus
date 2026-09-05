package store

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

//go:embed migrations/001_initial.sql
var migrationFiles embed.FS

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

func (p *Postgres) Migrate(ctx context.Context) error {
	migration, err := migrationFiles.ReadFile("migrations/001_initial.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := p.pool.Exec(ctx, string(migration)); err != nil {
		return fmt.Errorf("run migration: %w", err)
	}
	return nil
}

func (p *Postgres) CreateSite(ctx context.Context, site domain.Site) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO sites (id, name, slug, created_at)
		VALUES ($1, $2, $3, $4)
	`, site.ID, site.Name, site.Slug, site.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("create site: %w", err)
	}
	return nil
}

func (p *Postgres) GetSite(ctx context.Context, id string) (domain.Site, error) {
	var site domain.Site
	err := p.pool.QueryRow(ctx, `SELECT id, name, slug, created_at FROM sites WHERE id = $1`, id).
		Scan(&site.ID, &site.Name, &site.Slug, &site.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Site{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Site{}, fmt.Errorf("get site: %w", err)
	}
	return site, nil
}

func (p *Postgres) GetSiteBySlug(ctx context.Context, slug string) (domain.Site, error) {
	var site domain.Site
	err := p.pool.QueryRow(ctx, `SELECT id, name, slug, created_at FROM sites WHERE slug = $1`, slug).
		Scan(&site.ID, &site.Name, &site.Slug, &site.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Site{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Site{}, fmt.Errorf("get site by slug: %w", err)
	}
	return site, nil
}

func (p *Postgres) CreatePage(ctx context.Context, page domain.Page, version domain.PageVersion) error {
	root, err := marshalNode(page.Root)
	if err != nil {
		return err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create page: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO pages (id, site_id, name, slug, root, current_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, page.ID, page.SiteID, page.Name, page.Slug, root, page.Version, page.CreatedAt, page.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("insert page: %w", err)
	}
	if err := insertPageVersion(ctx, tx, version); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create page: %w", err)
	}
	return nil
}

func (p *Postgres) GetPage(ctx context.Context, id string) (domain.Page, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT id, site_id, name, slug, root, current_version, created_at, updated_at
		FROM pages WHERE id = $1
	`, id)
	page, err := scanPage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Page{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Page{}, fmt.Errorf("get page: %w", err)
	}
	return page, nil
}

func (p *Postgres) ListPagesBySite(ctx context.Context, siteID string) ([]domain.Page, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, name, slug, root, current_version, created_at, updated_at
		FROM pages WHERE site_id = $1 ORDER BY created_at, id
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Page, 0)
	for rows.Next() {
		page, err := scanPage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		result = append(result, page)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pages: %w", err)
	}
	return result, nil
}

func (p *Postgres) UpdatePage(ctx context.Context, page domain.Page, version domain.PageVersion) error {
	root, err := marshalNode(page.Root)
	if err != nil {
		return err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update page: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE pages
		SET root = $2, current_version = $3, updated_at = $4
		WHERE id = $1
	`, page.ID, root, page.Version, page.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update page: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	if err := insertPageVersion(ctx, tx, version); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit update page: %w", err)
	}
	return nil
}

func (p *Postgres) ListPageVersions(ctx context.Context, pageID string) ([]domain.PageVersion, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, page_id, number, root, created_at
		FROM page_versions WHERE page_id = $1 ORDER BY number
	`, pageID)
	if err != nil {
		return nil, fmt.Errorf("list page versions: %w", err)
	}
	defer rows.Close()
	result := make([]domain.PageVersion, 0)
	for rows.Next() {
		var version domain.PageVersion
		var root []byte
		if err := rows.Scan(&version.ID, &version.PageID, &version.Number, &root, &version.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan page version: %w", err)
		}
		if err := unmarshalNode(root, &version.Root); err != nil {
			return nil, err
		}
		result = append(result, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate page versions: %w", err)
	}
	if len(result) == 0 {
		var exists bool
		if err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pages WHERE id = $1)`, pageID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check page: %w", err)
		}
		if !exists {
			return nil, domain.ErrNotFound
		}
	}
	return result, nil
}

func (p *Postgres) CreateSnapshot(ctx context.Context, snapshot domain.Snapshot) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO snapshots (id, site_id, name, created_at)
		VALUES ($1, $2, $3, $4)
	`, snapshot.ID, snapshot.SiteID, snapshot.Name, snapshot.CreatedAt); err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	for _, page := range snapshot.Pages {
		if _, err := tx.Exec(ctx, `
			INSERT INTO snapshot_pages (snapshot_id, page_id, version_id, version)
			VALUES ($1, $2, $3, $4)
		`, snapshot.ID, page.PageID, page.VersionID, page.Version); err != nil {
			return fmt.Errorf("insert snapshot page: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create snapshot: %w", err)
	}
	return nil
}

func (p *Postgres) GetSnapshot(ctx context.Context, id string) (domain.Snapshot, error) {
	var snapshot domain.Snapshot
	if err := p.pool.QueryRow(ctx, `SELECT id, site_id, name, created_at FROM snapshots WHERE id = $1`, id).
		Scan(&snapshot.ID, &snapshot.SiteID, &snapshot.Name, &snapshot.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Snapshot{}, domain.ErrNotFound
		}
		return domain.Snapshot{}, fmt.Errorf("get snapshot: %w", err)
	}
	rows, err := p.pool.Query(ctx, `
		SELECT page_id, version_id, version
		FROM snapshot_pages WHERE snapshot_id = $1 ORDER BY page_id
	`, id)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("get snapshot pages: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var page domain.SnapshotPage
		if err := rows.Scan(&page.PageID, &page.VersionID, &page.Version); err != nil {
			return domain.Snapshot{}, fmt.Errorf("scan snapshot page: %w", err)
		}
		snapshot.Pages = append(snapshot.Pages, page)
	}
	return snapshot, rows.Err()
}

func (p *Postgres) ListSnapshotsBySite(ctx context.Context, siteID string) ([]domain.Snapshot, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, site_id, name, created_at FROM snapshots WHERE site_id = $1 ORDER BY created_at, id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Snapshot, 0)
	for rows.Next() {
		var snapshot domain.Snapshot
		if err := rows.Scan(&snapshot.ID, &snapshot.SiteID, &snapshot.Name, &snapshot.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		result = append(result, snapshot)
	}
	return result, rows.Err()
}

type rowScanner interface{ Scan(...any) error }

func scanPage(row rowScanner) (domain.Page, error) {
	var page domain.Page
	var root []byte
	if err := row.Scan(&page.ID, &page.SiteID, &page.Name, &page.Slug, &root, &page.Version, &page.CreatedAt, &page.UpdatedAt); err != nil {
		return domain.Page{}, err
	}
	if err := unmarshalNode(root, &page.Root); err != nil {
		return domain.Page{}, err
	}
	return page, nil
}

func insertPageVersion(ctx context.Context, tx pgx.Tx, version domain.PageVersion) error {
	root, err := marshalNode(version.Root)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO page_versions (id, page_id, number, root, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, version.ID, version.PageID, version.Number, root, version.CreatedAt); err != nil {
		return fmt.Errorf("insert page version: %w", err)
	}
	return nil
}

func marshalNode(node domain.ComponentNode) ([]byte, error) {
	data, err := json.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("marshal component tree: %w", err)
	}
	return data, nil
}

func unmarshalNode(data []byte, node *domain.ComponentNode) error {
	if err := json.Unmarshal(data, node); err != nil {
		return fmt.Errorf("unmarshal component tree: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
