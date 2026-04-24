package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type windsurfAccountRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewWindsurfAccountRepository(client *dbent.Client, sqlDB *sql.DB) service.WindsurfAccountRepository {
	return &windsurfAccountRepository{client: client, sql: sqlDB}
}

func (r *windsurfAccountRepository) Create(ctx context.Context, account *service.WindsurfAccount) error {
	query := `
		INSERT INTO windsurf_accounts (
			account,
			password_encrypted,
			enabled,
			maintained_by,
			maintained_at
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{
			account.Account,
			account.PasswordEncrypted,
			account.Enabled,
			account.MaintainedBy,
			account.MaintainedAt,
		},
		&account.ID,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return err
	}
	return nil
}

func (r *windsurfAccountRepository) Update(ctx context.Context, account *service.WindsurfAccount) error {
	query := `
		UPDATE windsurf_accounts
		SET account = $2,
			password_encrypted = $3,
			enabled = $4,
			maintained_by = $5,
			maintained_at = $6,
			status_updated_by = $7,
			status_updated_at = $8,
			updated_at = NOW()
		WHERE id = $1
		RETURNING created_at, updated_at
	`

	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{
			account.ID,
			account.Account,
			account.PasswordEncrypted,
			account.Enabled,
			account.MaintainedBy,
			account.MaintainedAt,
			nullInt64(account.StatusUpdatedBy),
			nullTimePtr(account.StatusUpdatedAt),
		},
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return translatePersistenceError(err, service.ErrWindsurfAccountNotFound, nil)
	}
	return nil
}

func (r *windsurfAccountRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM windsurf_accounts WHERE id = $1`
	result, err := r.sql.ExecContext(ctx, query, id)
	if err != nil {
		return translatePersistenceError(err, service.ErrWindsurfAccountNotFound, nil)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return service.ErrWindsurfAccountNotFound
	}
	return nil
}

func (r *windsurfAccountRepository) GetByID(ctx context.Context, id int64) (*service.WindsurfAccount, error) {
	query := `
		SELECT id, account, password_encrypted, enabled, maintained_by, maintained_at,
			status_updated_by, status_updated_at, created_at, updated_at
		FROM windsurf_accounts
		WHERE id = $1
	`

	item := &service.WindsurfAccount{}
	var statusUpdatedBy sql.NullInt64
	var statusUpdatedAt sql.NullTime
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{id},
		&item.ID,
		&item.Account,
		&item.PasswordEncrypted,
		&item.Enabled,
		&item.MaintainedBy,
		&item.MaintainedAt,
		&statusUpdatedBy,
		&statusUpdatedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, translatePersistenceError(err, service.ErrWindsurfAccountNotFound, nil)
	}
	if statusUpdatedBy.Valid {
		v := statusUpdatedBy.Int64
		item.StatusUpdatedBy = &v
	}
	if statusUpdatedAt.Valid {
		v := statusUpdatedAt.Time
		item.StatusUpdatedAt = &v
	}
	return item, nil
}

func (r *windsurfAccountRepository) List(
	ctx context.Context,
	params pagination.PaginationParams,
	filters service.WindsurfAccountListFilters,
) ([]service.WindsurfAccount, *pagination.PaginationResult, error) {
	where := ""
	args := make([]any, 0, 4)
	search := strings.TrimSpace(filters.Search)
	if search != "" {
		where = " WHERE account ILIKE $1 "
		args = append(args, "%"+search+"%")
	}

	countQuery := "SELECT COUNT(*) FROM windsurf_accounts" + where
	var total int64
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		return nil, nil, err
	}
	if total == 0 {
		return []service.WindsurfAccount{}, paginationResultFromTotal(0, params), nil
	}

	orderBy := "maintained_at DESC, id DESC"
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)
	switch sortBy {
	case "account":
		orderBy = buildOrderBy("account", sortOrder)
	case "enabled":
		orderBy = buildOrderBy("enabled", sortOrder)
	case "created_at":
		orderBy = buildOrderBy("created_at", sortOrder)
	case "updated_at":
		orderBy = buildOrderBy("updated_at", sortOrder)
	case "", "maintained_at":
		orderBy = buildOrderBy("maintained_at", sortOrder)
	}

	dataQuery := `
		SELECT id, account, password_encrypted, enabled, maintained_by, maintained_at,
			status_updated_by, status_updated_at, created_at, updated_at
		FROM windsurf_accounts` + where + `
		ORDER BY ` + orderBy + `
		LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.sql.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.WindsurfAccount, 0)
	for rows.Next() {
		item := service.WindsurfAccount{}
		var statusUpdatedBy sql.NullInt64
		var statusUpdatedAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.Account,
			&item.PasswordEncrypted,
			&item.Enabled,
			&item.MaintainedBy,
			&item.MaintainedAt,
			&statusUpdatedBy,
			&statusUpdatedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		if statusUpdatedBy.Valid {
			v := statusUpdatedBy.Int64
			item.StatusUpdatedBy = &v
		}
		if statusUpdatedAt.Valid {
			v := statusUpdatedAt.Time
			item.StatusUpdatedAt = &v
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return out, paginationResultFromTotal(total, params), nil
}

func buildOrderBy(column, sortOrder string) string {
	if sortOrder == pagination.SortOrderAsc {
		return column + " ASC, id ASC"
	}
	return column + " DESC, id DESC"
}

func nullTimePtr(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}
