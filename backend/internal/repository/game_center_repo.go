package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

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
	err := r.db.QueryRowContext(ctx, `SELECT balance, points FROM users WHERE id = $1`, userID).Scan(&assets.Balance, &assets.Points)
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
	if err := tx.QueryRowContext(ctx, `SELECT points FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&pointsBefore); err != nil {
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
	if _, err := tx.ExecContext(ctx, `UPDATE users SET points = $2, updated_at = NOW() WHERE id = $1`, input.UserID, pointsAfter); err != nil {
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

func (r *gameCenterRepository) ExchangeBalanceToPoints(ctx context.Context, input service.ExchangeBalanceToPointsInput) (*service.GameCenterExchangeResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var balanceBefore float64
	var pointsBefore int64
	if err := tx.QueryRowContext(ctx, `SELECT balance, points FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&balanceBefore, &pointsBefore); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}

	balanceAfter := balanceBefore - input.Amount
	pointsAfter := pointsBefore + input.TargetPoints
	if _, err := tx.ExecContext(ctx, `UPDATE users SET balance = $2, points = $3, updated_at = NOW() WHERE id = $1`, input.UserID, balanceAfter, pointsAfter); err != nil {
		return nil, err
	}

	var exchangeID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO game_points_exchanges (
			user_id, direction, source_amount, target_points, rate, status, reason
		) VALUES ($1, $2, $3, $4, $5, 'completed', 'balance to points')
		RETURNING id
	`, input.UserID, service.GameCenterExchangeDirectionBalanceToPoints, input.Amount, input.TargetPoints, input.Rate).Scan(&exchangeID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_points_ledger (
			user_id, entry_type, delta_points, points_before, points_after, related_exchange_id, reason
		) VALUES ($1, 'exchange_from_balance', $2, $3, $4, $5, 'balance to points')
	`, input.UserID, input.TargetPoints, pointsBefore, pointsAfter, exchangeID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &service.GameCenterExchangeResult{
		Direction:    service.GameCenterExchangeDirectionBalanceToPoints,
		SourceAmount: input.Amount,
		TargetPoints: input.TargetPoints,
		Rate:         input.Rate,
	}, nil
}

func (r *gameCenterRepository) ExchangePointsToBalance(ctx context.Context, input service.ExchangePointsToBalanceInput) (*service.GameCenterExchangeResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var balanceBefore float64
	var pointsBefore int64
	if err := tx.QueryRowContext(ctx, `SELECT balance, points FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&balanceBefore, &pointsBefore); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}

	balanceAfter := balanceBefore + input.TargetAmount
	pointsAfter := pointsBefore - input.Points
	if _, err := tx.ExecContext(ctx, `UPDATE users SET balance = $2, points = $3, updated_at = NOW() WHERE id = $1`, input.UserID, balanceAfter, pointsAfter); err != nil {
		return nil, err
	}

	var exchangeID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO game_points_exchanges (
			user_id, direction, source_points, target_amount, rate, status, reason
		) VALUES ($1, $2, $3, $4, $5, 'completed', 'points to balance')
		RETURNING id
	`, input.UserID, service.GameCenterExchangeDirectionPointsToBalance, input.Points, input.TargetAmount, input.Rate).Scan(&exchangeID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_points_ledger (
			user_id, entry_type, delta_points, points_before, points_after, related_exchange_id, reason
		) VALUES ($1, 'exchange_to_balance', $2, $3, $4, $5, 'points to balance')
	`, input.UserID, -input.Points, pointsBefore, pointsAfter, exchangeID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &service.GameCenterExchangeResult{
		Direction:    service.GameCenterExchangeDirectionPointsToBalance,
		SourcePoints: input.Points,
		TargetAmount: input.TargetAmount,
		Rate:         input.Rate,
	}, nil
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

	var items []service.GameCatalog
	for rows.Next() {
		var item service.GameCatalog
		if err := rows.Scan(
			&item.GameKey, &item.Name, &item.Subtitle, &item.CoverImage, &item.Description,
			&item.Enabled, &item.SortOrder, &item.DefaultOpenMode, &item.SupportsEmbed, &item.SupportsStandalone,
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

func (r *gameCenterRepository) ListLedger(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.GamePointsLedgerItem, *pagination.PaginationResult, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_points_ledger WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entry_type, delta_points, points_after, reason, created_at
		FROM game_points_ledger
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.GamePointsLedgerItem, 0)
	for rows.Next() {
		var item service.GamePointsLedgerItem
		if err := rows.Scan(&item.ID, &item.EntryType, &item.DeltaPoints, &item.PointsAfter, &item.Reason, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	pages := 1
	if params.PageSize > 0 && total > 0 {
		pages = int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	}
	if total == 0 {
		pages = 1
	}
	return items, &pagination.PaginationResult{
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    pages,
	}, nil
}

func (r *gameCenterRepository) ListClaimedBatchKeys(ctx context.Context, userID int64, claimDate string) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT batch_key
		FROM game_points_claims
		WHERE user_id = $1 AND claim_date = $2
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

func (r *gameCenterRepository) ListAdminLedger(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]service.GameCenterAdminLedgerItem, *pagination.PaginationResult, error) {
	queryArgs := []any{}
	where := ""
	if userID != nil {
		where = " WHERE user_id = $1"
		queryArgs = append(queryArgs, *userID)
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_points_ledger`+where, queryArgs...).Scan(&total); err != nil {
		return nil, nil, err
	}

	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, entry_type, delta_points, points_before, points_after, reason, related_game_key, created_at
		FROM game_points_ledger`+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+argPos(len(queryArgs)-1)+` OFFSET $`+argPos(len(queryArgs))+`
	`, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.GameCenterAdminLedgerItem, 0)
	for rows.Next() {
		var item service.GameCenterAdminLedgerItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.EntryType, &item.DeltaPoints, &item.PointsBefore, &item.PointsAfter, &item.Reason, &item.RelatedGameKey, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	return items, buildPageResult(total, params), rows.Err()
}

func (r *gameCenterRepository) ListClaimRecords(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]service.GameCenterClaimRecord, *pagination.PaginationResult, error) {
	queryArgs := []any{}
	where := ""
	if userID != nil {
		where = " WHERE user_id = $1"
		queryArgs = append(queryArgs, *userID)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_points_claims`+where, queryArgs...).Scan(&total); err != nil {
		return nil, nil, err
	}
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, claim_date::text, batch_key, points_amount, claimed_at
		FROM game_points_claims`+where+`
		ORDER BY claimed_at DESC, id DESC
		LIMIT $`+argPos(len(queryArgs)-1)+` OFFSET $`+argPos(len(queryArgs))+`
	`, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.GameCenterClaimRecord, 0)
	for rows.Next() {
		var item service.GameCenterClaimRecord
		if err := rows.Scan(&item.ID, &item.UserID, &item.ClaimDate, &item.BatchKey, &item.PointsAmount, &item.ClaimedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	return items, buildPageResult(total, params), rows.Err()
}

func (r *gameCenterRepository) ListExchangeRecords(ctx context.Context, params pagination.PaginationParams, userID *int64) ([]service.GameCenterExchangeRecord, *pagination.PaginationResult, error) {
	queryArgs := []any{}
	where := ""
	if userID != nil {
		where = " WHERE user_id = $1"
		queryArgs = append(queryArgs, *userID)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_points_exchanges`+where, queryArgs...).Scan(&total); err != nil {
		return nil, nil, err
	}
	queryArgs = append(queryArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, direction, source_amount, source_points, target_amount, target_points, rate, status, reason, created_at
		FROM game_points_exchanges`+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+argPos(len(queryArgs)-1)+` OFFSET $`+argPos(len(queryArgs))+`
	`, queryArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.GameCenterExchangeRecord, 0)
	for rows.Next() {
		var item service.GameCenterExchangeRecord
		if err := rows.Scan(&item.ID, &item.UserID, &item.Direction, &item.SourceAmount, &item.SourcePoints, &item.TargetAmount, &item.TargetPoints, &item.Rate, &item.Status, &item.Reason, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	return items, buildPageResult(total, params), rows.Err()
}

func (r *gameCenterRepository) AdjustPoints(ctx context.Context, input service.AdminAdjustPointsInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var before int64
	if err := tx.QueryRowContext(ctx, `SELECT points FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&before); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrUserNotFound
		}
		return err
	}

	after := before + input.DeltaPoints
	if after < 0 {
		return service.ErrGameCenterInsufficientPoints
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET points = $2, updated_at = NOW() WHERE id = $1`, input.UserID, after); err != nil {
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

func buildPageResult(total int64, params pagination.PaginationParams) *pagination.PaginationResult {
	pages := 1
	if params.PageSize > 0 && total > 0 {
		pages = int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	}
	if total == 0 {
		pages = 1
	}
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
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
