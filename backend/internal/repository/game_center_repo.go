package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type gameCenterRepository struct {
	db *sql.DB
}

func NewGameCenterRepository(_ *dbent.Client, sqlDB *sql.DB) service.GameCenterRepository {
	return &gameCenterRepository{db: sqlDB}
}

func (r *gameCenterRepository) GetUserAssets(ctx context.Context, userID int64) (*service.GameCenterAssets, error) {
	var assets service.GameCenterAssets
	err := r.db.QueryRowContext(ctx, `
		SELECT points
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&assets.Points)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	return &assets, err
}

func (r *gameCenterRepository) ClaimPoints(ctx context.Context, input service.ClaimPointsInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var pointsBefore int64
	if err := tx.QueryRowContext(ctx, `
		SELECT points
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, input.UserID).Scan(&pointsBefore); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrUserNotFound
		}
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_points_claims (user_id, claim_date, batch_key, points_amount, claimed_at)
		VALUES ($1, $2, $3, $4, $5)
	`, input.UserID, input.ClaimDate, input.BatchKey, input.PointsAmount, input.ClaimedAt); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return service.ErrGameCenterClaimAlreadyClaimed
		}
		return err
	}

	pointsAfter := pointsBefore + input.PointsAmount
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET points = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, input.UserID, pointsAfter); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_points_ledger (
			user_id, entry_type, delta_points, points_before, points_after, related_claim_batch_key, reason
		) VALUES ($1, 'daily_claim', $2, $3, $4, $5, 'daily claim')
	`, input.UserID, input.PointsAmount, pointsBefore, pointsAfter, input.BatchKey); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *gameCenterRepository) ListCatalogs(ctx context.Context) ([]service.GameCatalog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT game_key, name, subtitle, cover_image, description, enabled, sort_order, default_open_mode, supports_embed, supports_standalone
		FROM game_catalogs
		ORDER BY sort_order ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.GameCatalog, 0)
	for rows.Next() {
		var item service.GameCatalog
		if err := rows.Scan(
			&item.GameKey,
			&item.Name,
			&item.Subtitle,
			&item.CoverImage,
			&item.Description,
			&item.Enabled,
			&item.SortOrder,
			&item.DefaultOpenMode,
			&item.SupportsEmbed,
			&item.SupportsStandalone,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *gameCenterRepository) UpdateCatalog(ctx context.Context, gameKey string, req service.UpdateGameCatalogRequest) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE game_catalogs
		SET enabled = $2,
		    sort_order = $3,
		    default_open_mode = $4,
		    supports_embed = $5,
		    supports_standalone = $6,
		    updated_at = NOW()
		WHERE game_key = $1
	`, gameKey, req.Enabled, req.SortOrder, req.DefaultOpenMode, req.SupportsEmbed, req.SupportsStandalone)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrGameCenterCatalogNotFound
	}
	return nil
}

func (r *gameCenterRepository) ListClaimedBatchKeys(ctx context.Context, userID int64, claimDate string) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT batch_key
		FROM game_points_claims
		WHERE user_id = $1 AND claim_date = $2::date
	`, userID, claimDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]struct{})
	for rows.Next() {
		var batchKey string
		if err := rows.Scan(&batchKey); err != nil {
			return nil, err
		}
		result[batchKey] = struct{}{}
	}
	return result, rows.Err()
}

func (r *gameCenterRepository) ListLedger(ctx context.Context, params pagination.PaginationParams, filter service.GamePointsLedgerFilter) ([]service.GamePointsLedgerItem, *pagination.PaginationResult, error) {
	where, args := buildGamePointsWhere("gl", "created_at", filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_points_ledger gl`+where, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT gl.id, gl.user_id, COALESCE(u.email, ''), COALESCE(NULLIF(u.username, ''), u.email, CONCAT('user-', gl.user_id)),
		       gl.entry_type, gl.delta_points, gl.points_after, gl.reason, gl.created_at
		FROM game_points_ledger gl
		LEFT JOIN users u ON u.id = gl.user_id`+where+`
		ORDER BY gl.created_at DESC, gl.id DESC
		LIMIT $`+argPos(len(args)-1)+` OFFSET $`+argPos(len(args))+`
	`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.GamePointsLedgerItem, 0)
	for rows.Next() {
		var item service.GamePointsLedgerItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.Email, &item.Username, &item.EntryType, &item.DeltaPoints, &item.PointsAfter, &item.Reason, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, buildGameCenterPaginationResult(total, params), nil
}

func (r *gameCenterRepository) ListPointsLeaderboard(ctx context.Context, params pagination.PaginationParams) ([]service.GamePointsLeaderboardItem, *pagination.PaginationResult, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT ranked.rank, ranked.id, ranked.email, ranked.username, ranked.points
		FROM (
			SELECT
				ROW_NUMBER() OVER (ORDER BY points DESC, id ASC) AS rank,
				id,
				COALESCE(email, '') AS email,
				COALESCE(NULLIF(username, ''), email, CONCAT('user-', id)) AS username,
				points
			FROM users
			WHERE deleted_at IS NULL
		) ranked
		ORDER BY ranked.rank ASC
		LIMIT $1 OFFSET $2
	`, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.GamePointsLeaderboardItem, 0)
	for rows.Next() {
		var item service.GamePointsLeaderboardItem
		if err := rows.Scan(&item.Rank, &item.UserID, &item.Email, &item.Username, &item.Points); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	return items, buildGameCenterPaginationResult(total, params), rows.Err()
}

func (r *gameCenterRepository) ListAdminLedger(ctx context.Context, params pagination.PaginationParams, filter service.GamePointsLedgerFilter) ([]service.GameCenterAdminLedgerItem, *pagination.PaginationResult, error) {
	where, args := buildGamePointsWhere("gl", "created_at", filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_points_ledger gl`+where, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT gl.id, gl.user_id, COALESCE(u.email, ''), COALESCE(NULLIF(u.username, ''), u.email, CONCAT('user-', gl.user_id)),
		       gl.entry_type, gl.delta_points, gl.points_before, gl.points_after, gl.reason, gl.related_game_key, gl.created_at
		FROM game_points_ledger gl
		LEFT JOIN users u ON u.id = gl.user_id`+where+`
		ORDER BY gl.created_at DESC, gl.id DESC
		LIMIT $`+argPos(len(args)-1)+` OFFSET $`+argPos(len(args))+`
	`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.GameCenterAdminLedgerItem, 0)
	for rows.Next() {
		var item service.GameCenterAdminLedgerItem
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Email,
			&item.Username,
			&item.EntryType,
			&item.DeltaPoints,
			&item.PointsBefore,
			&item.PointsAfter,
			&item.Reason,
			&item.RelatedGameKey,
			&item.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	return items, buildGameCenterPaginationResult(total, params), rows.Err()
}

func (r *gameCenterRepository) ListClaimRecords(ctx context.Context, params pagination.PaginationParams, filter service.GamePointsLedgerFilter) ([]service.GameCenterClaimRecord, *pagination.PaginationResult, error) {
	where, args := buildGamePointsWhere("gpc", "claimed_at", filter)

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_points_claims gpc`+where, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT gpc.id, gpc.user_id, COALESCE(u.email, ''), COALESCE(NULLIF(u.username, ''), u.email, CONCAT('user-', gpc.user_id)),
		       gpc.claim_date::text, gpc.batch_key, gpc.points_amount, gpc.claimed_at
		FROM game_points_claims gpc
		LEFT JOIN users u ON u.id = gpc.user_id`+where+`
		ORDER BY gpc.claimed_at DESC, gpc.id DESC
		LIMIT $`+argPos(len(args)-1)+` OFFSET $`+argPos(len(args))+`
	`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.GameCenterClaimRecord, 0)
	for rows.Next() {
		var item service.GameCenterClaimRecord
		if err := rows.Scan(&item.ID, &item.UserID, &item.Email, &item.Username, &item.ClaimDate, &item.BatchKey, &item.PointsAmount, &item.ClaimedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	return items, buildGameCenterPaginationResult(total, params), rows.Err()
}

func (r *gameCenterRepository) AdjustPoints(ctx context.Context, input service.AdminAdjustPointsInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var before int64
	if err := tx.QueryRowContext(ctx, `
		SELECT points
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, input.UserID).Scan(&before); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrUserNotFound
		}
		return err
	}

	after := before + input.DeltaPoints
	if after < 0 {
		return service.ErrGameCenterInsufficientPoints
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET points = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, input.UserID, after); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_points_ledger (
			user_id, entry_type, delta_points, points_before, points_after, reason
		) VALUES ($1, 'admin_adjust', $2, $3, $4, $5)
	`, input.UserID, input.DeltaPoints, before, after, input.Reason); err != nil {
		return err
	}
	return tx.Commit()
}

func buildGameCenterPaginationResult(total int64, params pagination.PaginationParams) *pagination.PaginationResult {
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.Limit()
	pages := 1
	if total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &pagination.PaginationResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}
}

func argPos(pos int) string {
	return strconv.Itoa(pos)
}

func buildGamePointsWhere(alias, timeColumn string, filter service.GamePointsLedgerFilter) (string, []any) {
	column := func(name string) string {
		if strings.TrimSpace(alias) == "" {
			return name
		}
		return fmt.Sprintf("%s.%s", alias, name)
	}

	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if filter.UserID != nil && *filter.UserID > 0 {
		args = append(args, *filter.UserID)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column("user_id"), len(args)))
	}
	if filter.StartTime != nil {
		args = append(args, filter.StartTime.UTC())
		conditions = append(conditions, fmt.Sprintf("%s >= $%d", column(timeColumn), len(args)))
	}
	if filter.EndTime != nil {
		args = append(args, filter.EndTime.UTC())
		conditions = append(conditions, fmt.Sprintf("%s < $%d", column(timeColumn), len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}
