package service

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"time"
)

const (
	AffiliateTicketCampaignDailyCap      = 10
	AffiliateTicketCampaignRegisterPair  = 2
	AffiliateTicketCampaignRechargeFloor = 10.0
	AffiliateTicketCampaignRetentionDays = 2
	AffiliateTicketCampaignUsageFloor    = 20.0
	AffiliateTicketCampaignBalanceFloor  = 10.0
	AffiliateTicketCampaignInviteeBonus  = 1.0
)

var ErrAffiliateTicketCampaignRisk = errors.New("affiliate ticket campaign risk blocked")

type AffiliateTicketCampaignDaily struct {
	PlayDate         time.Time `json:"play_date"`
	RegisteredCount  int       `json:"registered_count"`
	RechargeCount    int       `json:"recharge_count"`
	TicketCount      int       `json:"ticket_count"`
	DailyCap         int       `json:"daily_cap"`
	TicketsRemaining int       `json:"tickets_remaining"`
}

type AffiliateTicketCampaignInvitee struct {
	UserID             int64      `json:"user_id"`
	Email              string     `json:"email"`
	Username           string     `json:"username"`
	RegisteredAt       *time.Time `json:"registered_at,omitempty"`
	RegistrationStatus string     `json:"registration_status"`
	RechargeQualified  bool       `json:"recharge_qualified"`
	TicketStatus       string     `json:"ticket_status"`
	RiskStatus         string     `json:"risk_status"`
}

type AffiliateTicketCampaignEligibility struct {
	Eligible                 bool    `json:"eligible"`
	HasUsageRecord           bool    `json:"has_usage_record"`
	HistoricalUsage          float64 `json:"historical_usage"`
	CurrentBalance           float64 `json:"current_balance"`
	HistoricalUsageThreshold float64 `json:"historical_usage_threshold"`
	BalanceThreshold         float64 `json:"balance_threshold"`
}

type AffiliateTicketCampaignDetail struct {
	Enabled                bool                               `json:"enabled"`
	Description            string                             `json:"description"`
	RegistrationPair       int                                `json:"registration_pair"`
	RechargeThreshold      float64                            `json:"recharge_threshold"`
	DailyCap               int                                `json:"daily_cap"`
	TicketRetentionDays    int                                `json:"ticket_retention_days"`
	ExistingTicketCapacity int                                `json:"existing_ticket_capacity"`
	InviteeBonus           float64                            `json:"invitee_bonus"`
	Eligibility            AffiliateTicketCampaignEligibility `json:"eligibility"`
	Daily                  AffiliateTicketCampaignDaily       `json:"daily"`
	Invitees               []AffiliateTicketCampaignInvitee   `json:"invitees"`
}

type AffiliateTicketCampaignEvent struct {
	ID           int64     `json:"id"`
	EventType    string    `json:"event_type"`
	InviterID    int64     `json:"inviter_id"`
	InviterEmail string    `json:"inviter_email"`
	InviteeID    int64     `json:"invitee_id"`
	InviteeEmail string    `json:"invitee_email"`
	OrderID      *int64    `json:"order_id,omitempty"`
	PlayDate     time.Time `json:"play_date"`
	Amount       float64   `json:"amount"`
	TicketCount  int       `json:"ticket_count"`
	Status       string    `json:"status"`
	RiskReason   string    `json:"risk_reason,omitempty"`
	InviterIP    string    `json:"inviter_ip,omitempty"`
	InviteeIP    string    `json:"invitee_ip,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type AffiliateTicketCampaignEventFilter struct {
	Search    string
	Status    string
	EventType string
	Page      int
	PageSize  int
	StartAt   *time.Time
	EndAt     *time.Time
}

type AffiliateTicketCampaignRepository interface {
	RecordRegistrationIP(context.Context, int64, string) error
	ProcessInviteRegistration(context.Context, int64, int64, string, time.Time) (*AffiliateTicketCampaignEvent, error)
	ProcessInviteRecharge(context.Context, int64, int64, float64, time.Time) (*AffiliateTicketCampaignEvent, error)
	GetEligibility(context.Context, int64) (*AffiliateTicketCampaignEligibility, error)
	GetDaily(context.Context, int64, time.Time) (*AffiliateTicketCampaignDaily, error)
	ListInvitees(context.Context, int64, int) ([]AffiliateTicketCampaignInvitee, error)
	ListEvents(context.Context, AffiliateTicketCampaignEventFilter) ([]AffiliateTicketCampaignEvent, int, error)
}

type AffiliateTicketCampaignService struct {
	repo           AffiliateTicketCampaignRepository
	settingService *SettingService
	clock          func() time.Time
}

func NewAffiliateTicketCampaignService(repo AffiliateTicketCampaignRepository, settingService *SettingService) *AffiliateTicketCampaignService {
	return &AffiliateTicketCampaignService{repo: repo, settingService: settingService, clock: time.Now}
}

func (s *AffiliateTicketCampaignService) SetClock(clock func() time.Time) {
	if clock != nil {
		s.clock = clock
	}
}

func (s *AffiliateTicketCampaignService) Enabled(ctx context.Context) bool {
	if s == nil || s.repo == nil {
		return false
	}
	return s.settingService == nil || s.settingService.IsAffiliateTicketCampaignEnabled(ctx)
}

func (s *AffiliateTicketCampaignService) RecordRegistrationIP(ctx context.Context, userID int64, rawIP string) error {
	if s == nil || s.repo == nil || userID <= 0 {
		return nil
	}
	return s.repo.RecordRegistrationIP(ctx, userID, normalizeCampaignIP(rawIP))
}

func (s *AffiliateTicketCampaignService) OnInviteRegistration(ctx context.Context, inviterID, inviteeID int64) error {
	if !s.Enabled(ctx) {
		return nil
	}
	event, err := s.repo.ProcessInviteRegistration(ctx, inviterID, inviteeID, registrationIPFromContext(ctx), campaignPlayDate(s.clock()))
	if err != nil {
		return err
	}
	if event != nil && event.Status == "blocked" {
		return ErrAffiliateTicketCampaignRisk
	}
	return nil
}

func (s *AffiliateTicketCampaignService) OnInviteRecharge(ctx context.Context, inviteeID, orderID int64, amount float64) error {
	if !s.Enabled(ctx) {
		return nil
	}
	_, err := s.repo.ProcessInviteRecharge(ctx, inviteeID, orderID, amount, campaignPlayDate(s.clock()))
	return err
}

func (s *AffiliateTicketCampaignService) GetDetail(ctx context.Context, inviterID int64) (*AffiliateTicketCampaignDetail, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	date := campaignPlayDate(s.clock())
	eligibility, err := s.repo.GetEligibility(ctx, inviterID)
	if err != nil {
		return nil, err
	}
	daily, err := s.repo.GetDaily(ctx, inviterID, date)
	if err != nil {
		return nil, err
	}
	invitees, err := s.repo.ListInvitees(ctx, inviterID, 100)
	if err != nil {
		return nil, err
	}
	return &AffiliateTicketCampaignDetail{
		Enabled:                s.Enabled(ctx),
		Description:            "满足活动参与条件后，邀请 2 位好友注册可获 1 张抽奖券；好友首次充值满 10 元可再获 1 张。",
		RegistrationPair:       AffiliateTicketCampaignRegisterPair,
		RechargeThreshold:      AffiliateTicketCampaignRechargeFloor,
		DailyCap:               AffiliateTicketCampaignDailyCap,
		TicketRetentionDays:    AffiliateTicketCampaignRetentionDays,
		ExistingTicketCapacity: ZenxiangLiyuTicketCapacity,
		InviteeBonus:           AffiliateTicketCampaignInviteeBonus,
		Eligibility:            *eligibility,
		Daily:                  *daily,
		Invitees:               invitees,
	}, nil
}

func (s *AffiliateTicketCampaignService) ListEvents(ctx context.Context, filter AffiliateTicketCampaignEventFilter) ([]AffiliateTicketCampaignEvent, int, error) {
	if s == nil || s.repo == nil {
		return []AffiliateTicketCampaignEvent{}, 0, nil
	}
	return s.repo.ListEvents(ctx, filter)
}

func campaignPlayDate(now time.Time) time.Time {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	local := now.In(shanghai)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

type campaignRegistrationIPContextKey struct{}

func WithRegistrationIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, campaignRegistrationIPContextKey{}, normalizeCampaignIP(ip))
}

func registrationIPFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(campaignRegistrationIPContextKey{}).(string)
	return normalizeCampaignIP(value)
}

func normalizeCampaignIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if addr, err := netip.ParseAddr(raw); err == nil {
		return addr.Unmap().String()
	}
	return ""
}
