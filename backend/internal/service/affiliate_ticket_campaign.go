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

type AffiliateTicketCampaignDetail struct {
	Enabled                bool                             `json:"enabled"`
	Description            string                           `json:"description"`
	RegistrationPair       int                              `json:"registration_pair"`
	RechargeThreshold      float64                          `json:"recharge_threshold"`
	DailyCap               int                              `json:"daily_cap"`
	TicketRetentionDays    int                              `json:"ticket_retention_days"`
	ExistingTicketCapacity int                              `json:"existing_ticket_capacity"`
	Daily                  AffiliateTicketCampaignDaily     `json:"daily"`
	Invitees               []AffiliateTicketCampaignInvitee `json:"invitees"`
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
	GetDaily(context.Context, int64, time.Time) (*AffiliateTicketCampaignDaily, error)
	ListInvitees(context.Context, int64, int) ([]AffiliateTicketCampaignInvitee, error)
	ListEvents(context.Context, AffiliateTicketCampaignEventFilter) ([]AffiliateTicketCampaignEvent, int, error)
}

type AffiliateTicketCampaignService struct {
	repo  AffiliateTicketCampaignRepository
	clock func() time.Time
}

func NewAffiliateTicketCampaignService(repo AffiliateTicketCampaignRepository) *AffiliateTicketCampaignService {
	return &AffiliateTicketCampaignService{repo: repo, clock: time.Now}
}

func (s *AffiliateTicketCampaignService) SetClock(clock func() time.Time) {
	if clock != nil {
		s.clock = clock
	}
}

func (s *AffiliateTicketCampaignService) Enabled(ctx context.Context) bool {
	return s != nil && s.repo != nil
}

func (s *AffiliateTicketCampaignService) RecordRegistrationIP(ctx context.Context, userID int64, rawIP string) error {
	if s == nil || s.repo == nil || userID <= 0 {
		return nil
	}
	return s.repo.RecordRegistrationIP(ctx, userID, normalizeCampaignIP(rawIP))
}

func (s *AffiliateTicketCampaignService) OnInviteRegistration(ctx context.Context, inviterID, inviteeID int64) error {
	if s == nil || s.repo == nil {
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
	if s == nil || s.repo == nil {
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
	daily, err := s.repo.GetDaily(ctx, inviterID, date)
	if err != nil {
		return nil, err
	}
	invitees, err := s.repo.ListInvitees(ctx, inviterID, 100)
	if err != nil {
		return nil, err
	}
	return &AffiliateTicketCampaignDetail{
		Enabled:                true,
		Description:            "邀请 2 位好友注册赠 1 张；每位好友首次充值满 10 元再赠 1 张；每日最多获得 10 张；活动券有效期 2 天。",
		RegistrationPair:       AffiliateTicketCampaignRegisterPair,
		RechargeThreshold:      AffiliateTicketCampaignRechargeFloor,
		DailyCap:               AffiliateTicketCampaignDailyCap,
		TicketRetentionDays:    AffiliateTicketCampaignRetentionDays,
		ExistingTicketCapacity: ZenxiangLiyuTicketCapacity,
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
	return raw
}
