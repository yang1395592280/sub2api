package repository

import (
	"context"
	"database/sql"
	"errors"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const redeemCampaignColumns = "id, name, per_user_limit, created_at, updated_at"

func scanRedeemCampaign(rows *sql.Rows) (*service.RedeemCampaign, error) {
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrRedeemCampaignNotFound
	}
	var campaign service.RedeemCampaign
	if err := rows.Scan(&campaign.ID, &campaign.Name, &campaign.PerUserLimit, &campaign.CreatedAt, &campaign.UpdatedAt); err != nil {
		return nil, err
	}
	return &campaign, nil
}

func (r *redeemCodeRepository) GetCampaign(ctx context.Context, id int64) (*service.RedeemCampaign, error) {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx,
		"SELECT "+redeemCampaignColumns+" FROM redeem_campaigns WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return scanRedeemCampaign(rows)
}

func (r *redeemCodeRepository) ListCampaigns(ctx context.Context, params pagination.PaginationParams, search string) ([]service.RedeemCampaign, int64, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, "SELECT COUNT(id) FROM redeem_campaigns WHERE STRPOS(LOWER(name), LOWER($1)) > 0", search)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err == nil {
			err = sql.ErrNoRows
		}
		return nil, 0, err
	}
	err = rows.Scan(&total)
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	rows, err = client.QueryContext(ctx, "SELECT "+redeemCampaignColumns+
		" FROM redeem_campaigns WHERE STRPOS(LOWER(name), LOWER($1)) > 0 ORDER BY id DESC LIMIT $2 OFFSET $3", search, params.Limit(), params.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	campaigns := make([]service.RedeemCampaign, 0)
	for rows.Next() {
		var campaign service.RedeemCampaign
		if err := rows.Scan(&campaign.ID, &campaign.Name, &campaign.PerUserLimit, &campaign.CreatedAt, &campaign.UpdatedAt); err != nil {
			return nil, 0, err
		}
		campaigns = append(campaigns, campaign)
	}
	return campaigns, total, rows.Err()
}

func (r *redeemCodeRepository) CreateCampaign(ctx context.Context, campaign *service.RedeemCampaign) error {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx,
		"INSERT INTO redeem_campaigns (name, per_user_limit) VALUES ($1, $2) RETURNING "+redeemCampaignColumns,
		campaign.Name, campaign.PerUserLimit)
	if err != nil {
		return err
	}
	created, err := scanRedeemCampaign(rows)
	if err == nil {
		*campaign = *created
	}
	return err
}

func (r *redeemCodeRepository) UpdateCampaign(ctx context.Context, campaign *service.RedeemCampaign) error {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx,
		"UPDATE redeem_campaigns SET name = $2, per_user_limit = $3, updated_at = NOW() WHERE id = $1 RETURNING "+redeemCampaignColumns,
		campaign.ID, campaign.Name, campaign.PerUserLimit)
	if err != nil {
		return err
	}
	updated, err := scanRedeemCampaign(rows)
	if err == nil {
		*campaign = *updated
	}
	return err
}

func (r *redeemCodeRepository) ClaimCampaign(ctx context.Context, campaignID, userID int64) error {
	if dbent.TxFromContext(ctx) == nil {
		return errors.New("redeem campaign claims require the redemption transaction")
	}
	client := clientFromContext(ctx, r.client)
	// Keep the limit stable until commit; administrators can change it between redemptions.
	rows, err := client.QueryContext(ctx, "SELECT per_user_limit FROM redeem_campaigns WHERE id = $1 FOR SHARE", campaignID)
	if err != nil {
		return err
	}
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		return service.ErrRedeemCampaignNotFound
	}
	var limit int
	err = rows.Scan(&limit)
	rows.Close()
	if err != nil {
		return err
	}
	// The unique campaign/user key serializes concurrent claims for different codes.
	// This counter survives code deletion and rolls back if benefit delivery fails.
	result, err := client.ExecContext(ctx, `
		INSERT INTO redeem_campaign_claims (campaign_id, user_id, redeemed_count)
		VALUES ($1, $2, 1)
		ON CONFLICT (campaign_id, user_id) DO UPDATE
		SET redeemed_count = redeem_campaign_claims.redeemed_count + 1
		WHERE redeem_campaign_claims.redeemed_count < $3`, campaignID, userID, limit)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return service.ErrRedeemCampaignLimit
	}
	return err
}
