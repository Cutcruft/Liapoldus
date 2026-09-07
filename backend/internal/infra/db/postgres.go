package db

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

//go:embed migrations/001_initial.sql migrations/002_admin_client_split.sql migrations/002_git_snapshots.sql migrations/003_component_definitions.sql migrations/004_builds.sql migrations/005_dependencies.sql migrations/006_dependency_allowlist.sql migrations/007_cache_config.sql migrations/008_tokens.sql migrations/009_component_source.sql migrations/010_operations.sql migrations/011_pages_list.sql migrations/012_deployments.sql migrations/013_site_settings.sql migrations/014_component_structure.sql
var migrationFiles embed.FS

type Postgres struct {
	pool *pgxpool.Pool
}

var _ domain.Storage = (*Postgres)(nil)

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
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		migration, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := p.pool.Exec(ctx, string(migration)); err != nil {
			return fmt.Errorf("run migration %s: %w", name, err)
		}
	}
	return nil
}

func (p *Postgres) CreateSite(ctx context.Context, site domain.Site) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO sites (id, name, slug, default_locale, hosts, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, site.ID, site.Name, site.Slug, site.DefaultLocale, site.Hosts, site.CreatedAt)
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
	err := p.pool.QueryRow(ctx, `SELECT id, name, slug, default_locale, hosts, created_at FROM sites WHERE id = $1`, id).
		Scan(&site.ID, &site.Name, &site.Slug, &site.DefaultLocale, &site.Hosts, &site.CreatedAt)
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
	err := p.pool.QueryRow(ctx, `SELECT id, name, slug, default_locale, hosts, created_at FROM sites WHERE slug = $1`, slug).
		Scan(&site.ID, &site.Name, &site.Slug, &site.DefaultLocale, &site.Hosts, &site.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Site{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Site{}, fmt.Errorf("get site by slug: %w", err)
	}
	return site, nil
}

func (p *Postgres) ListSites(ctx context.Context) ([]domain.Site, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, name, slug, default_locale, hosts, created_at FROM sites ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Site, 0)
	for rows.Next() {
		var site domain.Site
		if err := rows.Scan(&site.ID, &site.Name, &site.Slug, &site.DefaultLocale, &site.Hosts, &site.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan site: %w", err)
		}
		result = append(result, site)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateSite(ctx context.Context, site domain.Site) error {
	result, err := p.pool.Exec(ctx, `
		UPDATE sites SET name = $2, slug = $3, default_locale = $4, hosts = $5
		WHERE id = $1
	`, site.ID, site.Name, site.Slug, site.DefaultLocale, site.Hosts)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("update site: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteSite(ctx context.Context, id string) error {
	result, err := p.pool.Exec(ctx, `DELETE FROM sites WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete site: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) CreatePage(ctx context.Context, page domain.Page, version domain.PageVersion) error {
	list, err := marshalList(page.List)
	if err != nil {
		return err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create page: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO pages (id, site_id, name, slug, list, current_version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, page.ID, page.SiteID, page.Name, page.Slug, list, page.Version, page.CreatedAt, page.UpdatedAt); err != nil {
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
		SELECT id, site_id, name, slug, list, current_version, created_at, updated_at
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
		SELECT id, site_id, name, slug, list, current_version, created_at, updated_at
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
	list, err := marshalList(page.List)
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
		SET list = $2, current_version = $3, updated_at = $4
		WHERE id = $1
	`, page.ID, list, page.Version, page.UpdatedAt)
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
		SELECT id, page_id, number, list, created_at
		FROM page_versions WHERE page_id = $1 ORDER BY number
	`, pageID)
	if err != nil {
		return nil, fmt.Errorf("list page versions: %w", err)
	}
	defer rows.Close()
	result := make([]domain.PageVersion, 0)
	for rows.Next() {
		var version domain.PageVersion
		var list []byte
		if err := rows.Scan(&version.ID, &version.PageID, &version.Number, &list, &version.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan page version: %w", err)
		}
		if err := unmarshalList(list, &version.List); err != nil {
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
	lock, err := json.Marshal(snapshot.DepsLock)
	if err != nil {
		return fmt.Errorf("marshal snapshot lock: %w", err)
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO snapshots (id, site_id, name, deps_lock, git_sha, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, snapshot.ID, snapshot.SiteID, snapshot.Name, lock, snapshot.GitSHA, snapshot.CreatedAt); err != nil {
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
	var lock []byte
	if err := p.pool.QueryRow(ctx, `SELECT id, site_id, name, COALESCE(deps_lock, '{}'), COALESCE(git_sha, ''), created_at FROM snapshots WHERE id = $1`, id).
		Scan(&snapshot.ID, &snapshot.SiteID, &snapshot.Name, &lock, &snapshot.GitSHA, &snapshot.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Snapshot{}, domain.ErrNotFound
		}
		return domain.Snapshot{}, fmt.Errorf("get snapshot: %w", err)
	}
	pages, err := p.snapshotPages(ctx, id)
	if err != nil {
		return domain.Snapshot{}, err
	}
	snapshot.Pages = pages
	if err := json.Unmarshal(lock, &snapshot.DepsLock); err != nil {
		return domain.Snapshot{}, fmt.Errorf("unmarshal snapshot lock: %w", err)
	}
	return snapshot, nil
}

func (p *Postgres) snapshotPages(ctx context.Context, snapshotID string) ([]domain.SnapshotPage, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT page_id, version_id, version
		FROM snapshot_pages WHERE snapshot_id = $1 ORDER BY page_id
	`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot pages: %w", err)
	}
	defer rows.Close()
	pages := make([]domain.SnapshotPage, 0)
	for rows.Next() {
		var page domain.SnapshotPage
		if err := rows.Scan(&page.PageID, &page.VersionID, &page.Version); err != nil {
			return nil, fmt.Errorf("scan snapshot page: %w", err)
		}
		pages = append(pages, page)
	}
	return pages, rows.Err()
}

func (p *Postgres) ListSnapshotsBySite(ctx context.Context, siteID string) ([]domain.Snapshot, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, site_id, name, COALESCE(deps_lock, '{}'), COALESCE(git_sha, ''), created_at FROM snapshots WHERE site_id = $1 ORDER BY created_at, id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Snapshot, 0)
	for rows.Next() {
		var snapshot domain.Snapshot
		var lock []byte
		if err := rows.Scan(&snapshot.ID, &snapshot.SiteID, &snapshot.Name, &lock, &snapshot.GitSHA, &snapshot.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		if err := json.Unmarshal(lock, &snapshot.DepsLock); err != nil {
			return nil, fmt.Errorf("unmarshal snapshot lock: %w", err)
		}
		result = append(result, snapshot)
	}
	return result, rows.Err()
}

func (p *Postgres) GetPageVersion(ctx context.Context, pageID, versionID string) (domain.PageVersion, error) {
	var version domain.PageVersion
	var list []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id, page_id, number, list, created_at
		FROM page_versions WHERE page_id = $1 AND id = $2
	`, pageID, versionID).Scan(&version.ID, &version.PageID, &version.Number, &list, &version.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PageVersion{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PageVersion{}, fmt.Errorf("get page version: %w", err)
	}
	if err := unmarshalList(list, &version.List); err != nil {
		return domain.PageVersion{}, err
	}
	return version, nil
}

func (p *Postgres) DeletePage(ctx context.Context, id string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM pages WHERE id = $1`, id)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrInvalidRequest
		}
		return fmt.Errorf("delete page: %w", err)
	}
	return nil
}

func (p *Postgres) DeleteSnapshot(ctx context.Context, id string) error {
	result, err := p.pool.Exec(ctx, `DELETE FROM snapshots WHERE id = $1`, id)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrInvalidRequest
		}
		return fmt.Errorf("delete snapshot: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) scanDeployment(row pgx.Row) (domain.Deployment, error) {
	var deployment domain.Deployment
	if err := row.Scan(&deployment.ID, &deployment.SiteID, &deployment.Environment, &deployment.SnapshotID, &deployment.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Deployment{}, domain.ErrNotFound
		}
		return domain.Deployment{}, fmt.Errorf("scan deployment: %w", err)
	}
	return deployment, nil
}

func (p *Postgres) GetDeployment(ctx context.Context, siteID, environment string) (domain.Deployment, error) {
	return p.scanDeployment(p.pool.QueryRow(ctx, `
		SELECT id, site_id, environment, snapshot_id, created_at
		FROM deployments WHERE site_id = $1 AND environment = $2
	`, siteID, environment))
}

func (p *Postgres) ListDeploymentsBySite(ctx context.Context, siteID string) ([]domain.Deployment, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, environment, snapshot_id, created_at
		FROM deployments WHERE site_id = $1 ORDER BY environment
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Deployment, 0)
	for rows.Next() {
		var deployment domain.Deployment
		if err := rows.Scan(&deployment.ID, &deployment.SiteID, &deployment.Environment, &deployment.SnapshotID, &deployment.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan deployment: %w", err)
		}
		result = append(result, deployment)
	}
	return result, rows.Err()
}

func (p *Postgres) SetDeployment(ctx context.Context, deployment domain.Deployment) error {
	if _, err := p.pool.Exec(ctx, `
		INSERT INTO deployments (id, site_id, environment, snapshot_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (site_id, environment) DO UPDATE SET
			snapshot_id = EXCLUDED.snapshot_id,
			created_at = EXCLUDED.created_at
	`, deployment.ID, deployment.SiteID, deployment.Environment, deployment.SnapshotID, deployment.CreatedAt); err != nil {
		return fmt.Errorf("set deployment: %w", err)
	}
	return nil
}

func (p *Postgres) CreateContent(ctx context.Context, content domain.Content) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO contents (id, site_id, collection_id, key, fields, translations, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, content.ID, content.SiteID, content.CollectionID, content.Key, content.Fields, content.Translations, content.CreatedAt, content.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("create content: %w", err)
	}
	return nil
}

func (p *Postgres) GetContent(ctx context.Context, id string) (domain.Content, error) {
	return p.scanContent(ctx, p.pool.QueryRow(ctx, `
		SELECT id, site_id, collection_id, key, fields, translations, created_at, updated_at
		FROM contents WHERE id = $1
	`, id))
}

func (p *Postgres) ListContentsBySite(ctx context.Context, siteID, collectionID string) ([]domain.Content, error) {
	var rows pgx.Rows
	var err error
	if collectionID == "" {
		rows, err = p.pool.Query(ctx, `
			SELECT id, site_id, collection_id, key, fields, translations, created_at, updated_at
			FROM contents WHERE site_id = $1 ORDER BY collection_id, key
		`, siteID)
	} else {
		rows, err = p.pool.Query(ctx, `
			SELECT id, site_id, collection_id, key, fields, translations, created_at, updated_at
			FROM contents WHERE site_id = $1 AND collection_id = $2 ORDER BY key
		`, siteID, collectionID)
	}
	if err != nil {
		return nil, fmt.Errorf("list contents: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Content, 0)
	for rows.Next() {
		content, err := p.scanContent(ctx, rows)
		if err != nil {
			return nil, fmt.Errorf("scan content: %w", err)
		}
		result = append(result, content)
	}
	return result, rows.Err()
}

func (p *Postgres) GetContentsByIDs(ctx context.Context, siteID string, ids []string) (map[string]domain.Content, error) {
	if len(ids) == 0 {
		return map[string]domain.Content{}, nil
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, collection_id, key, fields, translations, created_at, updated_at
		FROM contents WHERE site_id = $1 AND id = ANY($2)
	`, siteID, ids)
	if err != nil {
		return nil, fmt.Errorf("get contents by ids: %w", err)
	}
	defer rows.Close()
	result := make(map[string]domain.Content)
	for rows.Next() {
		content, err := p.scanContent(ctx, rows)
		if err != nil {
			return nil, fmt.Errorf("scan content: %w", err)
		}
		result[content.ID] = content
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateContent(ctx context.Context, content domain.Content) error {
	result, err := p.pool.Exec(ctx, `
		UPDATE contents SET fields = $2, translations = $3, updated_at = $4
		WHERE id = $1
	`, content.ID, content.Fields, content.Translations, content.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update content: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteContent(ctx context.Context, id string) error {
	result, err := p.pool.Exec(ctx, `DELETE FROM contents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete content: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) scanContent(ctx context.Context, scanner pgx.Row) (domain.Content, error) {
	var content domain.Content
	if err := scanner.Scan(&content.ID, &content.SiteID, &content.CollectionID, &content.Key, &content.Fields, &content.Translations, &content.CreatedAt, &content.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Content{}, domain.ErrNotFound
		}
		return domain.Content{}, fmt.Errorf("scan content: %w", err)
	}
	return content, nil
}

func (p *Postgres) CreateAsset(ctx context.Context, asset domain.Asset) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO assets (id, site_id, name, mime, size, etag, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, asset.ID, asset.SiteID, asset.Name, asset.Mime, asset.Size, asset.ETag, asset.CreatedAt)
	if err != nil {
		return fmt.Errorf("create asset: %w", err)
	}
	return nil
}

func (p *Postgres) GetAsset(ctx context.Context, id string) (domain.Asset, error) {
	var asset domain.Asset
	err := p.pool.QueryRow(ctx, `
		SELECT id, site_id, name, mime, size, etag, created_at
		FROM assets WHERE id = $1
	`, id).Scan(&asset.ID, &asset.SiteID, &asset.Name, &asset.Mime, &asset.Size, &asset.ETag, &asset.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Asset{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Asset{}, fmt.Errorf("get asset: %w", err)
	}
	return asset, nil
}

func (p *Postgres) ListAssetsBySite(ctx context.Context, siteID string) ([]domain.Asset, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, name, mime, size, etag, created_at
		FROM assets WHERE site_id = $1 ORDER BY created_at, id
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Asset, 0)
	for rows.Next() {
		var asset domain.Asset
		if err := rows.Scan(&asset.ID, &asset.SiteID, &asset.Name, &asset.Mime, &asset.Size, &asset.ETag, &asset.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		result = append(result, asset)
	}
	return result, rows.Err()
}

func (p *Postgres) DeleteAsset(ctx context.Context, id string) error {
	result, err := p.pool.Exec(ctx, `DELETE FROM assets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) CreateRoute(ctx context.Context, route domain.Route) error {
	action, err := json.Marshal(route.Action)
	if err != nil {
		return fmt.Errorf("marshal route action: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO routes (id, site_id, matcher, priority, action, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, route.ID, route.SiteID, route.Matcher, route.Priority, action, route.CreatedAt, route.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create route: %w", err)
	}
	return nil
}

func (p *Postgres) GetRoute(ctx context.Context, siteID, id string) (domain.Route, error) {
	var route domain.Route
	var action []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id, site_id, matcher, priority, action, created_at, updated_at
		FROM routes WHERE id = $1 AND site_id = $2
	`, id, siteID).Scan(&route.ID, &route.SiteID, &route.Matcher, &route.Priority, &action, &route.CreatedAt, &route.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Route{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Route{}, fmt.Errorf("get route: %w", err)
	}
	if err := json.Unmarshal(action, &route.Action); err != nil {
		return domain.Route{}, fmt.Errorf("unmarshal route action: %w", err)
	}
	return route, nil
}

func (p *Postgres) ListRoutesBySite(ctx context.Context, siteID string) ([]domain.Route, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, matcher, priority, action, created_at, updated_at
		FROM routes WHERE site_id = $1 ORDER BY created_at, id
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Route, 0)
	for rows.Next() {
		var route domain.Route
		var action []byte
		if err := rows.Scan(&route.ID, &route.SiteID, &route.Matcher, &route.Priority, &action, &route.CreatedAt, &route.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		if err := json.Unmarshal(action, &route.Action); err != nil {
			return nil, fmt.Errorf("unmarshal route action: %w", err)
		}
		result = append(result, route)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateRoute(ctx context.Context, route domain.Route) error {
	action, err := json.Marshal(route.Action)
	if err != nil {
		return fmt.Errorf("marshal route action: %w", err)
	}
	result, err := p.pool.Exec(ctx, `
		UPDATE routes SET matcher = $2, priority = $3, action = $4, updated_at = $5
		WHERE id = $1 AND site_id = $6
	`, route.ID, route.Matcher, route.Priority, action, route.UpdatedAt, route.SiteID)
	if err != nil {
		return fmt.Errorf("update route: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteRoute(ctx context.Context, siteID, id string) error {
	result, err := p.pool.Exec(ctx, `DELETE FROM routes WHERE id = $1 AND site_id = $2`, id, siteID)
	if err != nil {
		return fmt.Errorf("delete route: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) CreateForm(ctx context.Context, form domain.Form) error {
	definition, err := json.Marshal(form.Definition)
	if err != nil {
		return fmt.Errorf("marshal form definition: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO forms (id, site_id, name, definition, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, form.ID, form.SiteID, form.Name, definition, form.CreatedAt, form.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create form: %w", err)
	}
	return nil
}

func (p *Postgres) GetForm(ctx context.Context, siteID, id string) (domain.Form, error) {
	var form domain.Form
	var definition []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id, site_id, name, definition, created_at, updated_at
		FROM forms WHERE id = $1 AND site_id = $2
	`, id, siteID).Scan(&form.ID, &form.SiteID, &form.Name, &definition, &form.CreatedAt, &form.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Form{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Form{}, fmt.Errorf("get form: %w", err)
	}
	if err := json.Unmarshal(definition, &form.Definition); err != nil {
		return domain.Form{}, fmt.Errorf("unmarshal form definition: %w", err)
	}
	return form, nil
}

func (p *Postgres) ListFormsBySite(ctx context.Context, siteID string) ([]domain.Form, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, name, definition, created_at, updated_at
		FROM forms WHERE site_id = $1 ORDER BY created_at, id
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list forms: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Form, 0)
	for rows.Next() {
		var form domain.Form
		var definition []byte
		if err := rows.Scan(&form.ID, &form.SiteID, &form.Name, &definition, &form.CreatedAt, &form.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan form: %w", err)
		}
		if err := json.Unmarshal(definition, &form.Definition); err != nil {
			return nil, fmt.Errorf("unmarshal form definition: %w", err)
		}
		result = append(result, form)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateForm(ctx context.Context, form domain.Form) error {
	definition, err := json.Marshal(form.Definition)
	if err != nil {
		return fmt.Errorf("marshal form definition: %w", err)
	}
	result, err := p.pool.Exec(ctx, `
		UPDATE forms SET name = $2, definition = $3, updated_at = $4
		WHERE id = $1 AND site_id = $5
	`, form.ID, form.Name, definition, form.UpdatedAt, form.SiteID)
	if err != nil {
		return fmt.Errorf("update form: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteForm(ctx context.Context, siteID, id string) error {
	result, err := p.pool.Exec(ctx, `DELETE FROM forms WHERE id = $1 AND site_id = $2`, id, siteID)
	if err != nil {
		return fmt.Errorf("delete form: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) CreateSubmission(ctx context.Context, submission domain.Submission) error {
	payload, err := json.Marshal(submission.Payload)
	if err != nil {
		return fmt.Errorf("marshal submission payload: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO submissions (id, site_id, form_id, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, submission.ID, submission.SiteID, submission.FormID, payload, submission.CreatedAt)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("create submission: %w", err)
	}
	return nil
}

func (p *Postgres) ListSubmissionsByForm(ctx context.Context, siteID, formID string) ([]domain.Submission, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, form_id, payload, created_at
		FROM submissions WHERE site_id = $1 AND form_id = $2 ORDER BY created_at, id
	`, siteID, formID)
	if err != nil {
		return nil, fmt.Errorf("list submissions: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Submission, 0)
	for rows.Next() {
		var submission domain.Submission
		if err := rows.Scan(&submission.ID, &submission.SiteID, &submission.FormID, &submission.Payload, &submission.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan submission: %w", err)
		}
		result = append(result, submission)
	}
	return result, rows.Err()
}

func (p *Postgres) DeleteSubmission(ctx context.Context, siteID, formID, submissionID string) error {
	result, err := p.pool.Exec(ctx, `
		DELETE FROM submissions WHERE id = $1 AND site_id = $2 AND form_id = $3
	`, submissionID, siteID, formID)
	if err != nil {
		return fmt.Errorf("delete submission: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanPage(row rowScanner) (domain.Page, error) {
	var page domain.Page
	var list []byte
	if err := row.Scan(&page.ID, &page.SiteID, &page.Name, &page.Slug, &list, &page.Version, &page.CreatedAt, &page.UpdatedAt); err != nil {
		return domain.Page{}, err
	}
	if err := unmarshalList(list, &page.List); err != nil {
		return domain.Page{}, err
	}
	return page, nil
}

func insertPageVersion(ctx context.Context, tx pgx.Tx, version domain.PageVersion) error {
	list, err := marshalList(version.List)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO page_versions (id, page_id, number, list, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, version.ID, version.PageID, version.Number, list, version.CreatedAt); err != nil {
		return fmt.Errorf("insert page version: %w", err)
	}
	return nil
}

func marshalList(list []domain.Element) ([]byte, error) {
	if list == nil {
		list = []domain.Element{}
	}
	data, err := json.Marshal(list)
	if err != nil {
		return nil, fmt.Errorf("marshal page elements: %w", err)
	}
	return data, nil
}

func unmarshalList(data []byte, list *[]domain.Element) error {
	if err := json.Unmarshal(data, list); err != nil {
		return fmt.Errorf("unmarshal page elements: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// --- ComponentDefinitionRepository -----------------------------------------

func (p *Postgres) Save(ctx context.Context, def *domain.ComponentDefinition) error {
	schemaJSON, err := json.Marshal(def.Schema)
	if err != nil {
		return fmt.Errorf("marshal definition schema: %w", err)
	}
	metadataJSON, err := json.Marshal(def.Metadata)
	if err != nil {
		return fmt.Errorf("marshal definition metadata: %w", err)
	}
	allowedJSON, err := json.Marshal(def.AllowedPrimitiveIDs)
	if err != nil {
		return fmt.Errorf("marshal definition allowed primitives: %w", err)
	}
	if _, err := p.pool.Exec(ctx, `
		INSERT INTO component_definitions (site_id, id, name, kind, is_section, allowed_primitive_ids, accepts_page_content, source, schema, metadata, current_sha, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (site_id, id) DO UPDATE SET
			name = EXCLUDED.name,
			kind = EXCLUDED.kind,
			is_section = EXCLUDED.is_section,
			allowed_primitive_ids = EXCLUDED.allowed_primitive_ids,
			accepts_page_content = EXCLUDED.accepts_page_content,
			source = EXCLUDED.source,
			schema = EXCLUDED.schema,
			metadata = EXCLUDED.metadata,
			current_sha = EXCLUDED.current_sha,
			updated_at = EXCLUDED.updated_at
	`, def.SiteID, def.ID, def.Name, def.Kind, def.IsSection, allowedJSON, def.AcceptsPageContent, def.Source, schemaJSON, metadataJSON, def.CurrentSHA, def.CreatedAt, def.UpdatedAt); err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("save definition: %w", err)
	}
	return nil
}

func (p *Postgres) Get(ctx context.Context, siteID, id string) (*domain.ComponentDefinition, error) {
	row := p.pool.QueryRow(ctx, `
		SELECT site_id, id, name, kind, is_section, allowed_primitive_ids, accepts_page_content, source, schema, metadata, current_sha, created_at, updated_at
		FROM component_definitions WHERE site_id = $1 AND id = $2
	`, siteID, id)
	def, err := scanDefinition(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get definition: %w", err)
	}
	return def, nil
}

func (p *Postgres) List(ctx context.Context, siteID string) ([]domain.ComponentDefinition, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT site_id, id, name, kind, is_section, allowed_primitive_ids, accepts_page_content, source, schema, metadata, current_sha, created_at, updated_at
		FROM component_definitions WHERE site_id = $1 ORDER BY id
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list definitions: %w", err)
	}
	defer rows.Close()
	result := make([]domain.ComponentDefinition, 0)
	for rows.Next() {
		def, err := scanDefinition(rows)
		if err != nil {
			return nil, fmt.Errorf("scan definition: %w", err)
		}
		result = append(result, *def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate definitions: %w", err)
	}
	return result, nil
}

func (p *Postgres) Delete(ctx context.Context, siteID, id string) error {
	result, err := p.pool.Exec(ctx, `DELETE FROM component_definitions WHERE site_id = $1 AND id = $2`, siteID, id)
	if err != nil {
		return fmt.Errorf("delete definition: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanDefinition(row rowScanner) (*domain.ComponentDefinition, error) {
	var def domain.ComponentDefinition
	var schemaJSON, metadataJSON, allowedJSON []byte
	if err := row.Scan(&def.SiteID, &def.ID, &def.Name, &def.Kind, &def.IsSection, &allowedJSON, &def.AcceptsPageContent, &def.Source, &schemaJSON, &metadataJSON, &def.CurrentSHA, &def.CreatedAt, &def.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(schemaJSON, &def.Schema); err != nil {
		return nil, fmt.Errorf("unmarshal definition schema: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &def.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal definition metadata: %w", err)
	}
	if err := json.Unmarshal(allowedJSON, &def.AllowedPrimitiveIDs); err != nil {
		return nil, fmt.Errorf("unmarshal definition allowed primitives: %w", err)
	}
	return &def, nil
}

// --- BuildRepository --------------------------------------------------------

var _ domain.BuildRepository = (*Postgres)(nil)

func (p *Postgres) CreateBuild(ctx context.Context, build domain.Build) error {
	logJSON, err := json.Marshal(build.Log)
	if err != nil {
		return fmt.Errorf("marshal build log: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO builds (id, site_id, snapshot_id, environment, status, log, artifact_dir, created_at, started_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (site_id, environment, snapshot_id, status) DO NOTHING
	`, build.ID, build.SiteID, build.SnapshotID, build.Environment, string(build.Status), logJSON, build.ArtifactDir, build.CreatedAt, build.StartedAt, build.FinishedAt)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("create build: %w", err)
	}
	return nil
}

func (p *Postgres) GetBuild(ctx context.Context, id string) (domain.Build, error) {
	build, err := scanBuild(p.pool.QueryRow(ctx, `
		SELECT id, site_id, snapshot_id, environment, status, log, artifact_dir, created_at, started_at, finished_at
		FROM builds WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Build{}, domain.ErrNotFound
		}
		return domain.Build{}, fmt.Errorf("get build: %w", err)
	}
	return build, nil
}

func (p *Postgres) ListBuildsBySite(ctx context.Context, siteID string) ([]domain.Build, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, snapshot_id, environment, status, log, artifact_dir, created_at, started_at, finished_at
		FROM builds WHERE site_id = $1 ORDER BY created_at, id
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list builds: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Build, 0)
	for rows.Next() {
		build, err := scanBuild(rows)
		if err != nil {
			return nil, fmt.Errorf("scan build: %w", err)
		}
		result = append(result, build)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate builds: %w", err)
	}
	return result, nil
}

func (p *Postgres) GetBuildBySnapshot(ctx context.Context, siteID, environment, snapshotID string) (domain.Build, error) {
	build, err := scanBuild(p.pool.QueryRow(ctx, `
		SELECT id, site_id, snapshot_id, environment, status, log, artifact_dir, created_at, started_at, finished_at
		FROM builds WHERE site_id = $1 AND environment = $2 AND snapshot_id = $3
		ORDER BY created_at DESC, id DESC LIMIT 1
	`, siteID, environment, snapshotID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Build{}, domain.ErrNotFound
		}
		return domain.Build{}, fmt.Errorf("get build by snapshot: %w", err)
	}
	return build, nil
}

func (p *Postgres) UpdateBuild(ctx context.Context, build domain.Build) error {
	logJSON, err := json.Marshal(build.Log)
	if err != nil {
		return fmt.Errorf("marshal build log: %w", err)
	}
	result, err := p.pool.Exec(ctx, `
		UPDATE builds SET status = $1, log = $2, artifact_dir = $3, started_at = $4, finished_at = $5
		WHERE id = $6
	`, string(build.Status), logJSON, build.ArtifactDir, build.StartedAt, build.FinishedAt, build.ID)
	if err != nil {
		return fmt.Errorf("update build: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanBuild(row rowScanner) (domain.Build, error) {
	var build domain.Build
	var statusRaw string
	var logJSON []byte
	if err := row.Scan(&build.ID, &build.SiteID, &build.SnapshotID, &build.Environment, &statusRaw, &logJSON, &build.ArtifactDir, &build.CreatedAt, &build.StartedAt, &build.FinishedAt); err != nil {
		return domain.Build{}, err
	}
	build.Status = domain.BuildStatus(statusRaw)
	if len(logJSON) > 0 {
		if err := json.Unmarshal(logJSON, &build.Log); err != nil {
			return domain.Build{}, fmt.Errorf("unmarshal build log: %w", err)
		}
	}
	return build, nil
}

func (p *Postgres) CreateDependency(ctx context.Context, dep domain.Dependency) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO site_dependencies (site_id, name, spec, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, dep.SiteID, dep.Name, dep.Spec, dep.CreatedAt, dep.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("create dependency: %w", err)
	}
	return nil
}

func (p *Postgres) GetDependency(ctx context.Context, siteID, name string) (domain.Dependency, error) {
	var dep domain.Dependency
	err := p.pool.QueryRow(ctx, `
		SELECT site_id, name, spec, created_at, updated_at
		FROM site_dependencies WHERE site_id = $1 AND name = $2
	`, siteID, name).Scan(&dep.SiteID, &dep.Name, &dep.Spec, &dep.CreatedAt, &dep.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Dependency{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Dependency{}, fmt.Errorf("get dependency: %w", err)
	}
	return dep, nil
}

func (p *Postgres) ListDependenciesBySite(ctx context.Context, siteID string) ([]domain.Dependency, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT site_id, name, spec, created_at, updated_at
		FROM site_dependencies WHERE site_id = $1 ORDER BY name
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list dependencies: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Dependency, 0)
	for rows.Next() {
		var dep domain.Dependency
		if err := rows.Scan(&dep.SiteID, &dep.Name, &dep.Spec, &dep.CreatedAt, &dep.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		result = append(result, dep)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateDependency(ctx context.Context, dep domain.Dependency) error {
	tag, err := p.pool.Exec(ctx, `
		UPDATE site_dependencies SET spec = $3, updated_at = $4
		WHERE site_id = $1 AND name = $2
	`, dep.SiteID, dep.Name, dep.Spec, dep.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update dependency: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteDependency(ctx context.Context, siteID, name string) error {
	tag, err := p.pool.Exec(ctx, `
		DELETE FROM site_dependencies WHERE site_id = $1 AND name = $2
	`, siteID, name)
	if err != nil {
		return fmt.Errorf("delete dependency: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) ListAllowlist(ctx context.Context, siteID string) ([]string, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT entry FROM site_dependency_allowlist WHERE site_id = $1 ORDER BY entry
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list allowlist: %w", err)
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var entry string
		if err := rows.Scan(&entry); err != nil {
			return nil, fmt.Errorf("scan allowlist entry: %w", err)
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (p *Postgres) AddAllowlist(ctx context.Context, siteID, entry string) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO site_dependency_allowlist (site_id, entry, created_at)
		VALUES ($1, $2, $3)
	`, siteID, entry, time.Now().UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("add allowlist entry: %w", err)
	}
	return nil
}

func (p *Postgres) RemoveAllowlist(ctx context.Context, siteID, entry string) error {
	tag, err := p.pool.Exec(ctx, `
		DELETE FROM site_dependency_allowlist WHERE site_id = $1 AND entry = $2
	`, siteID, entry)
	if err != nil {
		return fmt.Errorf("remove allowlist entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) SetCacheConfig(ctx context.Context, cfg domain.SiteCacheConfig) error {
	now := time.Now().UTC()
	_, err := p.pool.Exec(ctx, `
		INSERT INTO site_cache_config (site_id, max_deps_bytes, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (site_id) DO UPDATE SET max_deps_bytes = EXCLUDED.max_deps_bytes, updated_at = EXCLUDED.updated_at
	`, cfg.SiteID, cfg.MaxDepsBytes, now)
	if err != nil {
		return fmt.Errorf("set cache config: %w", err)
	}
	return nil
}

func (p *Postgres) GetCacheConfig(ctx context.Context, siteID string) (domain.SiteCacheConfig, bool, error) {
	var cfg domain.SiteCacheConfig
	err := p.pool.QueryRow(ctx, `
		SELECT site_id, max_deps_bytes FROM site_cache_config WHERE site_id = $1
	`, siteID).Scan(&cfg.SiteID, &cfg.MaxDepsBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SiteCacheConfig{}, false, nil
	}
	if err != nil {
		return domain.SiteCacheConfig{}, false, fmt.Errorf("get cache config: %w", err)
	}
	return cfg, true, nil
}

func (p *Postgres) ListCacheConfigs(ctx context.Context) ([]domain.SiteCacheConfig, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT site_id, max_deps_bytes FROM site_cache_config ORDER BY site_id
	`)
	if err != nil {
		return nil, fmt.Errorf("list cache configs: %w", err)
	}
	defer rows.Close()
	result := make([]domain.SiteCacheConfig, 0)
	for rows.Next() {
		var cfg domain.SiteCacheConfig
		if err := rows.Scan(&cfg.SiteID, &cfg.MaxDepsBytes); err != nil {
			return nil, fmt.Errorf("scan cache config: %w", err)
		}
		result = append(result, cfg)
	}
	return result, rows.Err()
}

func (p *Postgres) TouchTarballAccess(ctx context.Context, name, version string) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO dep_tarball_access (name, version, last_access)
		VALUES ($1, $2, $3)
		ON CONFLICT (name, version) DO UPDATE SET last_access = EXCLUDED.last_access
	`, name, version, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("touch tarball access: %w", err)
	}
	return nil
}

func (p *Postgres) ListTarballAccess(ctx context.Context) ([]domain.TarballAccess, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT name, version, last_access FROM dep_tarball_access ORDER BY last_access, name
	`)
	if err != nil {
		return nil, fmt.Errorf("list tarball access: %w", err)
	}
	defer rows.Close()
	result := make([]domain.TarballAccess, 0)
	for rows.Next() {
		var acc domain.TarballAccess
		if err := rows.Scan(&acc.Name, &acc.Version, &acc.LastAccess); err != nil {
			return nil, fmt.Errorf("scan tarball access: %w", err)
		}
		result = append(result, acc)
	}
	return result, rows.Err()
}

func (p *Postgres) GetDepPackage(ctx context.Context, name, version string) (domain.DepPackage, error) {
	var pkg domain.DepPackage
	var depsJSON []byte
	err := p.pool.QueryRow(ctx, `
		SELECT name, version, integrity, tarball_url, dependencies, fetched_at
		FROM dep_packages WHERE name = $1 AND version = $2
	`, name, version).
		Scan(&pkg.Name, &pkg.Version, &pkg.Integrity, &pkg.TarballURL, &depsJSON, &pkg.FetchedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DepPackage{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.DepPackage{}, fmt.Errorf("get dep package: %w", err)
	}
	if len(depsJSON) > 0 {
		if err := json.Unmarshal(depsJSON, &pkg.Dependencies); err != nil {
			return domain.DepPackage{}, fmt.Errorf("unmarshal dep package deps: %w", err)
		}
	}
	return pkg, nil
}

func (p *Postgres) CreateDepPackage(ctx context.Context, pkg domain.DepPackage) error {
	depsJSON, err := json.Marshal(pkg.Dependencies)
	if err != nil {
		return fmt.Errorf("marshal dep package deps: %w", err)
	}
	if _, err := p.pool.Exec(ctx, `
		INSERT INTO dep_packages (name, version, integrity, tarball_url, dependencies, fetched_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name, version) DO NOTHING
	`, pkg.Name, pkg.Version, pkg.Integrity, pkg.TarballURL, depsJSON, pkg.FetchedAt); err != nil {
		return fmt.Errorf("create dep package: %w", err)
	}
	return nil
}

func (p *Postgres) GetTokens(ctx context.Context, siteID string) (*domain.TokenSet, error) {
	var raw []byte
	err := p.pool.QueryRow(ctx, `
		SELECT tokens FROM site_tokens WHERE site_id = $1
	`, siteID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tokens: %w", err)
	}
	var tokens domain.TokenSet
	if err := json.Unmarshal(raw, &tokens); err != nil {
		return nil, fmt.Errorf("unmarshal tokens: %w", err)
	}
	return &tokens, nil
}

func (p *Postgres) UpsertTokens(ctx context.Context, siteID string, tokens *domain.TokenSet) error {
	raw, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}
	now := time.Now().UTC()
	if _, err := p.pool.Exec(ctx, `
		INSERT INTO site_tokens (site_id, tokens, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (site_id) DO UPDATE SET tokens = EXCLUDED.tokens, updated_at = EXCLUDED.updated_at
	`, siteID, raw, now); err != nil {
		return fmt.Errorf("upsert tokens: %w", err)
	}
	return nil
}

func (p *Postgres) GetSiteSettings(ctx context.Context, siteID string) (*domain.SiteSettings, error) {
	var raw []byte
	err := p.pool.QueryRow(ctx, `SELECT settings FROM site_settings WHERE site_id = $1`, siteID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get site settings: %w", err)
	}
	var settings domain.SiteSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("unmarshal site settings: %w", err)
	}
	return &settings, nil
}

func (p *Postgres) UpsertSiteSettings(ctx context.Context, settings *domain.SiteSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal site settings: %w", err)
	}
	if _, err := p.pool.Exec(ctx, `
		INSERT INTO site_settings (site_id, settings, updated_at) VALUES ($1, $2, $3)
		ON CONFLICT (site_id) DO UPDATE SET settings = EXCLUDED.settings, updated_at = EXCLUDED.updated_at
	`, settings.SiteID, raw, time.Now().UTC()); err != nil {
		return fmt.Errorf("upsert site settings: %w", err)
	}
	return nil
}
