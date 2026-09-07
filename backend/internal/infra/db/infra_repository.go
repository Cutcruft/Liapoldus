package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Operation/Endpoint repository implementation (R6). System rows have
// site_id IS NULL; Get resolves a site row first, otherwise falls back to the
// matching system row. Create rejects ids that already exist anywhere
// (system or site), so a site can never shadow a platform builtin.

func (p *Postgres) CreateOperation(ctx context.Context, op domain.Operation) error {
	params, err := json.Marshal(orEmpty(op.Params))
	if err != nil {
		return fmt.Errorf("marshal operation params: %w", err)
	}
	poll, err := json.Marshal(orEmpty(op.Poll))
	if err != nil {
		return fmt.Errorf("marshal operation poll: %w", err)
	}
	subscribe, err := json.Marshal(orEmpty(op.Subscribe))
	if err != nil {
		return fmt.Errorf("marshal operation subscribe: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO operations
			(id, site_id, system, provider, type_op, method, path, cache, ttl, scope, result_type, params, poll, subscribe, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, op.ID, op.SiteID, op.System, op.Provider, op.TypeOp, op.Method, op.Path, op.Cache, op.TTL, op.Scope, op.ResultType, params, poll, subscribe, op.CreatedAt, op.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("create operation: %w", err)
	}
	return nil
}

func (p *Postgres) GetOperation(ctx context.Context, siteID, id string) (domain.Operation, error) {
	op, err := scanOperation(p.pool.QueryRow(ctx, `
		SELECT id, site_id, system, provider, type_op, method, path, cache, ttl, scope, result_type, params, poll, subscribe, created_at, updated_at
		FROM operations WHERE id = $1 AND (site_id = $2 OR site_id IS NULL)
		ORDER BY site_id DESC NULLS LAST
		LIMIT 1
	`, id, siteID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Operation{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Operation{}, fmt.Errorf("get operation: %w", err)
	}
	return op, nil
}

func (p *Postgres) ListOperationsBySite(ctx context.Context, siteID string) ([]domain.Operation, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, system, provider, type_op, method, path, cache, ttl, scope, result_type, params, poll, subscribe, created_at, updated_at
		FROM operations WHERE site_id = $1 OR site_id IS NULL
		ORDER BY id
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Operation, 0)
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan operation: %w", err)
		}
		result = append(result, op)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateOperation(ctx context.Context, op domain.Operation) error {
	params, err := json.Marshal(orEmpty(op.Params))
	if err != nil {
		return fmt.Errorf("marshal operation params: %w", err)
	}
	poll, err := json.Marshal(orEmpty(op.Poll))
	if err != nil {
		return fmt.Errorf("marshal operation poll: %w", err)
	}
	subscribe, err := json.Marshal(orEmpty(op.Subscribe))
	if err != nil {
		return fmt.Errorf("marshal operation subscribe: %w", err)
	}
	result, err := p.pool.Exec(ctx, `
		UPDATE operations SET
			provider = $2, type_op = $3, method = $4, path = $5, cache = $6, ttl = $7,
			scope = $8, result_type = $9, params = $10, poll = $11, subscribe = $12, updated_at = $13
		WHERE id = $1 AND site_id = $14
	`, op.ID, op.Provider, op.TypeOp, op.Method, op.Path, op.Cache, op.TTL, op.Scope, op.ResultType, params, poll, subscribe, op.UpdatedAt, op.SiteID)
	if err != nil {
		return fmt.Errorf("update operation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteOperation(ctx context.Context, siteID, id string) error {
	result, err := p.pool.Exec(ctx, `
		DELETE FROM operations WHERE id = $1 AND site_id = $2
	`, id, siteID)
	if err != nil {
		return fmt.Errorf("delete operation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) CreateEndpoint(ctx context.Context, ep domain.Endpoint) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO endpoints (id, site_id, system, method, path, operation_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, ep.ID, ep.SiteID, ep.System, ep.Method, ep.Path, ep.OperationID, ep.CreatedAt, ep.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("create endpoint: %w", err)
	}
	return nil
}

func (p *Postgres) GetEndpoint(ctx context.Context, siteID, id string) (domain.Endpoint, error) {
	var ep domain.Endpoint
	err := p.pool.QueryRow(ctx, `
		SELECT id, site_id, system, method, path, operation_id, created_at, updated_at
		FROM endpoints WHERE id = $1 AND (site_id = $2 OR site_id IS NULL)
		ORDER BY site_id DESC NULLS LAST
		LIMIT 1
	`, id, siteID).Scan(&ep.ID, &ep.SiteID, &ep.System, &ep.Method, &ep.Path, &ep.OperationID, &ep.CreatedAt, &ep.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Endpoint{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Endpoint{}, fmt.Errorf("get endpoint: %w", err)
	}
	return ep, nil
}

func (p *Postgres) ListEndpointsBySite(ctx context.Context, siteID string) ([]domain.Endpoint, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, site_id, system, method, path, operation_id, created_at, updated_at
		FROM endpoints WHERE site_id = $1 OR site_id IS NULL
		ORDER BY id
	`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Endpoint, 0)
	for rows.Next() {
		var ep domain.Endpoint
		if err := rows.Scan(&ep.ID, &ep.SiteID, &ep.System, &ep.Method, &ep.Path, &ep.OperationID, &ep.CreatedAt, &ep.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan endpoint: %w", err)
		}
		result = append(result, ep)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateEndpoint(ctx context.Context, ep domain.Endpoint) error {
	result, err := p.pool.Exec(ctx, `
		UPDATE endpoints SET method = $2, path = $3, operation_id = $4, updated_at = $5
		WHERE id = $1 AND site_id = $6
	`, ep.ID, ep.Method, ep.Path, ep.OperationID, ep.UpdatedAt, ep.SiteID)
	if err != nil {
		return fmt.Errorf("update endpoint: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (p *Postgres) DeleteEndpoint(ctx context.Context, siteID, id string) error {
	result, err := p.pool.Exec(ctx, `
		DELETE FROM endpoints WHERE id = $1 AND site_id = $2
	`, id, siteID)
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type operationScanner interface{ Scan(...any) error }

func scanOperation(row operationScanner) (domain.Operation, error) {
	var op domain.Operation
	var params, poll, subscribe []byte
	err := row.Scan(&op.ID, &op.SiteID, &op.System, &op.Provider, &op.TypeOp, &op.Method, &op.Path,
		&op.Cache, &op.TTL, &op.Scope, &op.ResultType, &params, &poll, &subscribe, &op.CreatedAt, &op.UpdatedAt)
	if err != nil {
		return domain.Operation{}, err
	}
	if err := json.Unmarshal(params, &op.Params); err != nil {
		return domain.Operation{}, fmt.Errorf("unmarshal operation params: %w", err)
	}
	if err := json.Unmarshal(poll, &op.Poll); err != nil {
		return domain.Operation{}, fmt.Errorf("unmarshal operation poll: %w", err)
	}
	if err := json.Unmarshal(subscribe, &op.Subscribe); err != nil {
		return domain.Operation{}, fmt.Errorf("unmarshal operation subscribe: %w", err)
	}
	if op.Params == nil {
		op.Params = map[string]any{}
	}
	if op.Poll == nil {
		op.Poll = map[string]any{}
	}
	if op.Subscribe == nil {
		op.Subscribe = map[string]any{}
	}
	return op, nil
}

func orEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}