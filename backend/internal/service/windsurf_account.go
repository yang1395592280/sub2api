package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrWindsurfAccountNotFound           = infraerrors.NotFound("WINDSURF_ACCOUNT_NOT_FOUND", "windsurf account not found")
	ErrWindsurfAccountStatusAdminOnly    = infraerrors.Forbidden("WINDSURF_ACCOUNT_STATUS_ADMIN_ONLY", "only admin can update windsurf account status")
	ErrWindsurfAccountDeleteAdminOnly    = infraerrors.Forbidden("WINDSURF_ACCOUNT_DELETE_ADMIN_ONLY", "only admin can delete windsurf account")
	ErrWindsurfAccountPasswordViewDenied = infraerrors.Forbidden("WINDSURF_ACCOUNT_PASSWORD_VIEW_DENIED", "only the maintainer or an admin can view the windsurf password")
	ErrWindsurfAccountAccountRequired    = infraerrors.BadRequest("WINDSURF_ACCOUNT_REQUIRED", "windsurf account is required")
	ErrWindsurfAccountPasswordRequired   = infraerrors.BadRequest("WINDSURF_PASSWORD_REQUIRED", "windsurf password is required")
)

type WindsurfAccount struct {
	ID                int64
	Account           string
	PasswordEncrypted string
	Enabled           bool
	MaintainedBy      int64
	MaintainedAt      time.Time
	StatusUpdatedBy   *int64
	StatusUpdatedAt   *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type WindsurfAccountListFilters struct {
	Search string
}

type WindsurfAccountListItem struct {
	ID                int64      `json:"id"`
	Account           string     `json:"account"`
	PasswordMasked    string     `json:"password_masked"`
	Enabled           bool       `json:"enabled"`
	MaintainedByID    int64      `json:"maintained_by_id"`
	MaintainedByName  string     `json:"maintained_by_name"`
	MaintainedByEmail string     `json:"maintained_by_email"`
	MaintainedAt      time.Time  `json:"maintained_at"`
	StatusUpdatedBy   *int64     `json:"status_updated_by,omitempty"`
	StatusUpdatedAt   *time.Time `json:"status_updated_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type WindsurfAccountRepository interface {
	Create(ctx context.Context, account *WindsurfAccount) error
	Update(ctx context.Context, account *WindsurfAccount) error
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*WindsurfAccount, error)
	List(ctx context.Context, params pagination.PaginationParams, filters WindsurfAccountListFilters) ([]WindsurfAccount, *pagination.PaginationResult, error)
}

type CreateWindsurfAccountInput struct {
	Account  string
	Password string
	ActorID  int64
}

type UpdateWindsurfAccountCredentialsInput struct {
	Account  string
	Password string
	ActorID  int64
}

type UpdateWindsurfAccountStatusInput struct {
	Enabled bool
	ActorID int64
	IsAdmin bool
}

type DeleteWindsurfAccountInput struct {
	ActorID int64
	IsAdmin bool
}

type RevealWindsurfAccountPasswordInput struct {
	ActorID int64
	IsAdmin bool
}

func normalizeWindsurfAccountValue(value string) string {
	return strings.TrimSpace(value)
}

func maskWindsurfPassword(_ string) string {
	return "••••••"
}
