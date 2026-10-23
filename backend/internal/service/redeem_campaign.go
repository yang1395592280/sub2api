package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrRedeemCampaignNotFound = infraerrors.NotFound("REDEEM_CAMPAIGN_NOT_FOUND", "redeem campaign not found")
	ErrRedeemCampaignLimit    = infraerrors.Conflict("REDEEM_CAMPAIGN_LIMIT_REACHED", "redemption limit for this campaign reached")
)

type RedeemCampaign struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	PerUserLimit int       `json:"per_user_limit"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RedeemCampaignInput struct {
	Name         string `json:"name"`
	PerUserLimit int    `json:"per_user_limit"`
}

// RedeemCampaignRepository uses the same transaction as code consumption and benefit delivery.
type RedeemCampaignRepository interface {
	GetCampaign(context.Context, int64) (*RedeemCampaign, error)
	ListCampaigns(context.Context, pagination.PaginationParams, string) ([]RedeemCampaign, int64, error)
	CreateCampaign(context.Context, *RedeemCampaign) error
	UpdateCampaign(context.Context, *RedeemCampaign) error
	ClaimCampaign(context.Context, int64, int64) error
}

func validateRedeemCampaignInput(input *RedeemCampaignInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 100 || input.PerUserLimit < 1 || input.PerUserLimit > 1000 {
		return infraerrors.BadRequest("REDEEM_CAMPAIGN_INVALID", "campaign name must contain 1 to 100 characters and per_user_limit must be between 1 and 1000")
	}
	return nil
}

func redeemCampaignRepository(repo RedeemCodeRepository) (RedeemCampaignRepository, error) {
	campaignRepo, ok := repo.(RedeemCampaignRepository)
	if !ok {
		return nil, errors.New("redeem campaign repository is not configured")
	}
	return campaignRepo, nil
}

func validateRedeemCampaignCode(ctx context.Context, repo RedeemCodeRepository, campaignID *int64, codeType string, value float64) error {
	if campaignID == nil {
		return nil
	}
	if *campaignID <= 0 || (codeType != RedeemTypeBalance && codeType != RedeemTypeLotteryTicket) || value <= 0 {
		return infraerrors.BadRequest("REDEEM_CAMPAIGN_CODE_INVALID", "campaigns only support positive balance codes and lottery ticket codes")
	}
	campaignRepo, err := redeemCampaignRepository(repo)
	if err != nil {
		return err
	}
	_, err = campaignRepo.GetCampaign(ctx, *campaignID)
	return err
}

func (s *RedeemService) ListCampaigns(ctx context.Context, params pagination.PaginationParams, search string) ([]RedeemCampaign, int64, error) {
	repo, err := redeemCampaignRepository(s.redeemRepo)
	if err != nil {
		return nil, 0, err
	}
	return repo.ListCampaigns(ctx, params, strings.TrimSpace(search))
}

func (s *RedeemService) GetCampaign(ctx context.Context, id int64) (*RedeemCampaign, error) {
	repo, err := redeemCampaignRepository(s.redeemRepo)
	if err != nil {
		return nil, err
	}
	return repo.GetCampaign(ctx, id)
}

func (s *RedeemService) CreateCampaign(ctx context.Context, input RedeemCampaignInput) (*RedeemCampaign, error) {
	if err := validateRedeemCampaignInput(&input); err != nil {
		return nil, err
	}
	repo, err := redeemCampaignRepository(s.redeemRepo)
	if err != nil {
		return nil, err
	}
	campaign := &RedeemCampaign{Name: input.Name, PerUserLimit: input.PerUserLimit}
	if err := repo.CreateCampaign(ctx, campaign); err != nil {
		return nil, err
	}
	return campaign, nil
}

func (s *RedeemService) UpdateCampaign(ctx context.Context, id int64, input RedeemCampaignInput) (*RedeemCampaign, error) {
	if err := validateRedeemCampaignInput(&input); err != nil {
		return nil, err
	}
	repo, err := redeemCampaignRepository(s.redeemRepo)
	if err != nil {
		return nil, err
	}
	campaign := &RedeemCampaign{ID: id, Name: input.Name, PerUserLimit: input.PerUserLimit}
	if err := repo.UpdateCampaign(ctx, campaign); err != nil {
		return nil, err
	}
	return campaign, nil
}
