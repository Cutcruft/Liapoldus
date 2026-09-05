package db

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

//go:embed migrations/001_initial.sql migrations/002_admin_client_split.sql
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

func (p *Postgres) GetPageVersion(ctx context.Context, pageID, versionID string) (domain.PageVersion, error) {
	var version domain.PageVersion
	var root []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id, page_id, number, root, created_at
		FROM page_versions WHERE page_id = $1 AND id = $2
	`, pageID, versionID).Scan(&version.ID, &version.PageID, &version.Number, &root, &version.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PageVersion{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PageVersion{}, fmt.Errorf("get page version: %w", err)
	}
	if err := unmarshalNode(root, &version.Root); err != nil {
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
		return fmt.Errorf("delete snapshot: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
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

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
