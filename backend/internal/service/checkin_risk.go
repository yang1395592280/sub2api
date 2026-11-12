package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var ErrCheckinSuspended = infraerrors.Forbidden("CHECKIN_SUSPENDED", "签到福利因自动化领取违规已清退，签到及翻倍暂停；如有异议请联系管理员复核")

type CheckinRiskCase struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"user_id"`
	Status        string     `json:"status"`
	Reason        string     `json:"reason"`
	Evidence      string     `json:"evidence"`
	ObserveUntil  time.Time  `json:"observe_until"`
	ClearedAmount float64    `json:"cleared_amount"`
	CreatedAt     time.Time  `json:"created_at"`
	DecidedAt     *time.Time `json:"decided_at"`
}

type CheckinRiskNotice struct {
	CaseID        int64      `json:"case_id"`
	Reason        string     `json:"reason"`
	ClearedAmount float64    `json:"cleared_amount"`
	DecidedAt     *time.Time `json:"decided_at"`
}

type CheckinRiskRepository interface {
	GetCheckinRisk(context.Context, int64) (*CheckinRiskCase, error)
	ConfirmCheckinRisk(context.Context, int64, int64, string, string) (*CheckinRiskCase, error)
	ResolveCheckinRisk(context.Context, int64, int64, string) (int64, error)
	ListCheckinRisks(context.Context, int, int) ([]CheckinRiskCase, int64, error)
	ProcessDueCheckinRisks(context.Context) ([]int64, error)
	RecordCheckinSecurityEvent(context.Context, int64, string, string, string) error
	CreateCheckinPageToken(context.Context, int64, string) error
	ConsumeCheckinPageToken(context.Context, int64, string) (bool, error)
}

func (s *CheckinService) Risk(ctx context.Context, userID int64) (*CheckinRiskCase, error) {
	if r, ok := s.repo.(CheckinRiskRepository); ok {
		return r.GetCheckinRisk(ctx, userID)
	}
	return nil, nil
}

func (s *CheckinService) ListRisks(ctx context.Context, page, size int) ([]CheckinRiskCase, int64, error) {
	return s.repo.(CheckinRiskRepository).ListCheckinRisks(ctx, page, size)
}

func checkinTokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (s *CheckinService) CreatePageToken(ctx context.Context, userID int64) (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b[:])
	return token, s.repo.(CheckinRiskRepository).CreateCheckinPageToken(ctx, userID, checkinTokenHash(token))
}

func (s *CheckinService) CheckPageToken(ctx context.Context, userID int64, token string) (bool, error) {
	if len(token) != 64 {
		return false, nil
	}
	return s.repo.(CheckinRiskRepository).ConsumeCheckinPageToken(ctx, userID, checkinTokenHash(token))
}

func (s *CheckinService) ConfirmRisk(ctx context.Context, userID, actorID int64, reason, evidence string) (*CheckinRiskCase, error) {
	if strings.TrimSpace(reason) == "" || strings.TrimSpace(evidence) == "" || len(reason) > 2000 || len(evidence) > 8000 {
		return nil, infraerrors.BadRequest("CHECKIN_RISK_EVIDENCE_REQUIRED", "请填写违规原因和已核实的自动化证据")
	}
	return s.repo.(CheckinRiskRepository).ConfirmCheckinRisk(ctx, userID, actorID, reason, evidence)
}

func (s *CheckinService) ResolveRisk(ctx context.Context, caseID, actorID int64, action string) error {
	if action != "revoke" && action != "restore" {
		return infraerrors.BadRequest("CHECKIN_RISK_INVALID_ACTION", "invalid risk action")
	}
	userID, err := s.repo.(CheckinRiskRepository).ResolveCheckinRisk(ctx, caseID, actorID, action)
	if err != nil {
		return err
	}
	s.invalidateCaches(ctx, userID)
	return nil
}

func (s *CheckinService) SecurityEvent(ctx context.Context, userID int64, event, ip, userAgent string) error {
	if len(userAgent) > 500 {
		userAgent = userAgent[:500]
	}
	return s.repo.(CheckinRiskRepository).RecordCheckinSecurityEvent(ctx, userID, event, ip, userAgent)
}

func (s *CheckinService) RunRiskSweep(ctx context.Context) error {
	ids, err := s.repo.(CheckinRiskRepository).ProcessDueCheckinRisks(ctx)
	for _, id := range ids {
		s.invalidateCaches(ctx, id)
	}
	return err
}

func (s *CheckinService) Start() {
	s.riskStop = make(chan struct{})
	s.riskDone = make(chan struct{})
	go func() {
		defer close(s.riskDone)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := s.RunRiskSweep(ctx); err != nil {
				logger.LegacyPrintf("service.checkin", "risk sweep failed: %v", err)
			}
			cancel()
			select {
			case <-s.riskStop:
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *CheckinService) Stop() {
	if s != nil && s.riskStop != nil {
		s.riskStopOnce.Do(func() { close(s.riskStop) })
		<-s.riskDone
	}
}
