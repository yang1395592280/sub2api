package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type windsurfAccountRepoStub struct {
	items     []WindsurfAccount
	total     int64
	created   *WindsurfAccount
	updated   *WindsurfAccount
	byID      map[int64]*WindsurfAccount
	lastPage  pagination.PaginationParams
	deletedID int64
}

func (s *windsurfAccountRepoStub) Create(_ context.Context, account *WindsurfAccount) error {
	cloned := *account
	s.created = &cloned
	if cloned.ID == 0 {
		cloned.ID = 1
	}
	account.ID = cloned.ID
	account.CreatedAt = time.Now()
	account.UpdatedAt = account.CreatedAt
	if s.byID == nil {
		s.byID = map[int64]*WindsurfAccount{}
	}
	s.byID[account.ID] = account
	return nil
}

func (s *windsurfAccountRepoStub) Update(_ context.Context, account *WindsurfAccount) error {
	cloned := *account
	s.updated = &cloned
	if s.byID == nil {
		s.byID = map[int64]*WindsurfAccount{}
	}
	s.byID[account.ID] = account
	return nil
}

func (s *windsurfAccountRepoStub) GetByID(_ context.Context, id int64) (*WindsurfAccount, error) {
	if item, ok := s.byID[id]; ok {
		cloned := *item
		return &cloned, nil
	}
	return nil, ErrWindsurfAccountNotFound
}

func (s *windsurfAccountRepoStub) List(_ context.Context, params pagination.PaginationParams, _ WindsurfAccountListFilters) ([]WindsurfAccount, *pagination.PaginationResult, error) {
	s.lastPage = params
	cloned := make([]WindsurfAccount, 0, len(s.items))
	for i := range s.items {
		cloned = append(cloned, s.items[i])
	}
	return cloned, &pagination.PaginationResult{
		Total:    s.total,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

func (s *windsurfAccountRepoStub) Delete(_ context.Context, id int64) error {
	s.deletedID = id
	if s.byID != nil {
		delete(s.byID, id)
	}
	return nil
}

type windsurfUserRepoStub struct {
	users map[int64]*User
}

func (s *windsurfUserRepoStub) Create(context.Context, *User) error { panic("unexpected call") }
func (s *windsurfUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	if user, ok := s.users[id]; ok {
		cloned := *user
		return &cloned, nil
	}
	return nil, ErrUserNotFound
}
func (s *windsurfUserRepoStub) GetByEmail(context.Context, string) (*User, error) {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) GetFirstAdmin(context.Context) (*User, error) {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) Update(context.Context, *User) error { panic("unexpected call") }
func (s *windsurfUserRepoStub) Delete(context.Context, int64) error { panic("unexpected call") }
func (s *windsurfUserRepoStub) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected call")
}
func (s *windsurfUserRepoStub) EnableTotp(context.Context, int64) error  { panic("unexpected call") }
func (s *windsurfUserRepoStub) DisableTotp(context.Context, int64) error { panic("unexpected call") }

type windsurfEncryptorStub struct{}

func (windsurfEncryptorStub) Encrypt(plaintext string) (string, error) {
	return "enc:" + plaintext, nil
}

func (windsurfEncryptorStub) Decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, "enc:") {
		return "", fmt.Errorf("unexpected ciphertext format: %q", ciphertext)
	}
	return strings.TrimPrefix(ciphertext, "enc:"), nil
}

func TestWindsurfAccountServiceCreateDisablesAccountAndTracksMaintainer(t *testing.T) {
	repo := &windsurfAccountRepoStub{}
	userRepo := &windsurfUserRepoStub{
		users: map[int64]*User{
			7: {ID: 7, Username: "alice", Email: "alice@example.com"},
		},
	}
	svc := NewWindsurfAccountService(repo, userRepo, windsurfEncryptorStub{})

	item, err := svc.Create(context.Background(), &CreateWindsurfAccountInput{
		Account:  "windsurf@example.com",
		Password: "super-secret",
		ActorID:  7,
	})

	require.NoError(t, err)
	require.NotNil(t, repo.created)
	require.Equal(t, "windsurf@example.com", repo.created.Account)
	require.Equal(t, "aesgcm:enc:super-secret", repo.created.PasswordEncrypted)
	require.False(t, repo.created.Enabled)
	require.Equal(t, int64(7), repo.created.MaintainedBy)
	require.WithinDuration(t, time.Now(), repo.created.MaintainedAt, 2*time.Second)
	require.Equal(t, "alice", item.MaintainedByName)
	require.Equal(t, "alice@example.com", item.MaintainedByEmail)
	require.NotEmpty(t, item.PasswordMasked)
	require.NotEqual(t, "super-secret", item.PasswordMasked)
}

func TestWindsurfAccountServiceUpdateCredentialsResetsEnabledStatus(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			9: {
				ID:                9,
				Account:           "old@example.com",
				PasswordEncrypted: "enc:old-password",
				Enabled:           true,
				MaintainedBy:      1,
				MaintainedAt:      now.Add(-time.Hour),
			},
		},
	}
	userRepo := &windsurfUserRepoStub{
		users: map[int64]*User{
			8: {ID: 8, Username: "bob", Email: "bob@example.com"},
		},
	}
	svc := NewWindsurfAccountService(repo, userRepo, windsurfEncryptorStub{})

	item, err := svc.UpdateCredentials(context.Background(), 9, &UpdateWindsurfAccountCredentialsInput{
		Account:  "new@example.com",
		Password: "new-password",
		ActorID:  8,
		IsAdmin:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, repo.updated)
	require.Equal(t, "new@example.com", repo.updated.Account)
	require.Equal(t, "aesgcm:enc:new-password", repo.updated.PasswordEncrypted)
	require.False(t, repo.updated.Enabled)
	require.Equal(t, int64(8), repo.updated.MaintainedBy)
	require.WithinDuration(t, time.Now(), repo.updated.MaintainedAt, 2*time.Second)
	require.Equal(t, "bob", item.MaintainedByName)
}

func TestWindsurfAccountServiceUpdateCredentialsAllowsMaintainerToChangePasswordOnly(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			9: {
				ID:                9,
				Account:           "owner@example.com",
				PasswordEncrypted: "enc:old-password",
				Enabled:           true,
				MaintainedBy:      8,
				MaintainedAt:      now.Add(-time.Hour),
			},
		},
	}
	userRepo := &windsurfUserRepoStub{
		users: map[int64]*User{
			8: {ID: 8, Username: "bob", Email: "bob@example.com"},
		},
	}
	svc := NewWindsurfAccountService(repo, userRepo, windsurfEncryptorStub{})

	item, err := svc.UpdateCredentials(context.Background(), 9, &UpdateWindsurfAccountCredentialsInput{
		Account:  "should-not-change@example.com",
		Password: "new-password",
		ActorID:  8,
		IsAdmin:  false,
	})

	require.NoError(t, err)
	require.NotNil(t, repo.updated)
	require.Equal(t, "owner@example.com", repo.updated.Account)
	require.Equal(t, "aesgcm:enc:new-password", repo.updated.PasswordEncrypted)
	require.False(t, repo.updated.Enabled)
	require.Equal(t, int64(8), repo.updated.MaintainedBy)
	require.Equal(t, "owner@example.com", item.Account)
}

func TestWindsurfAccountServiceUpdateCredentialsRejectsNonMaintainer(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			9: {
				ID:                9,
				Account:           "owner@example.com",
				PasswordEncrypted: "enc:old-password",
				MaintainedBy:      8,
			},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	item, err := svc.UpdateCredentials(context.Background(), 9, &UpdateWindsurfAccountCredentialsInput{
		Account:  "other@example.com",
		Password: "new-password",
		ActorID:  7,
		IsAdmin:  false,
	})

	require.ErrorIs(t, err, ErrWindsurfAccountUpdateDenied)
	require.Nil(t, item)
	require.Nil(t, repo.updated)
}

func TestWindsurfAccountServiceUpdateCredentialsRequiresPasswordForNonAdmin(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			9: {
				ID:                9,
				Account:           "owner@example.com",
				PasswordEncrypted: "enc:old-password",
				MaintainedBy:      8,
			},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	item, err := svc.UpdateCredentials(context.Background(), 9, &UpdateWindsurfAccountCredentialsInput{
		Account: "owner@example.com",
		ActorID: 8,
		IsAdmin: false,
	})

	require.ErrorIs(t, err, ErrWindsurfAccountPasswordRequired)
	require.Nil(t, item)
	require.Nil(t, repo.updated)
}

func TestWindsurfAccountServiceUpdateStatusRequiresAdmin(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			3: {ID: 3, Account: "windsurf@example.com", PasswordEncrypted: "enc:secret"},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	_, err := svc.UpdateStatus(context.Background(), 3, &UpdateWindsurfAccountStatusInput{
		Enabled: true,
		ActorID: 2,
		IsAdmin: false,
	})

	require.ErrorIs(t, err, ErrWindsurfAccountStatusAdminOnly)
}

func TestWindsurfAccountServiceRevealPasswordRejectsNonMaintainer(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			5: {ID: 5, Account: "windsurf@example.com", PasswordEncrypted: "enc:secret-value", MaintainedBy: 7},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	password, err := svc.RevealPassword(context.Background(), 5, &RevealWindsurfAccountPasswordInput{
		ActorID: 8,
		IsAdmin: false,
	})

	require.ErrorIs(t, err, ErrWindsurfAccountPasswordViewDenied)
	require.Empty(t, password)
}

func TestWindsurfAccountServiceRevealPasswordAllowsMaintainer(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			5: {ID: 5, Account: "windsurf@example.com", PasswordEncrypted: "enc:secret-value", MaintainedBy: 7},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	password, err := svc.RevealPassword(context.Background(), 5, &RevealWindsurfAccountPasswordInput{
		ActorID: 7,
		IsAdmin: false,
	})

	require.NoError(t, err)
	require.Equal(t, "secret-value", password)
}

func TestWindsurfAccountServiceRevealPasswordAllowsAdmin(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			5: {ID: 5, Account: "windsurf@example.com", PasswordEncrypted: "enc:secret-value", MaintainedBy: 7},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	password, err := svc.RevealPassword(context.Background(), 5, &RevealWindsurfAccountPasswordInput{
		ActorID: 99,
		IsAdmin: true,
	})

	require.NoError(t, err)
	require.Equal(t, "secret-value", password)
}

func TestWindsurfAccountServiceRevealPasswordReadsPrefixedEncryptedPassword(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			5: {ID: 5, Account: "windsurf@example.com", PasswordEncrypted: "aesgcm:enc:secret-value", MaintainedBy: 7},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	password, err := svc.RevealPassword(context.Background(), 5, &RevealWindsurfAccountPasswordInput{
		ActorID: 99,
		IsAdmin: true,
	})

	require.NoError(t, err)
	require.Equal(t, "secret-value", password)
	require.Nil(t, repo.updated)
}

func TestWindsurfAccountServiceRevealPasswordMigratesLegacyPlaintext(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			6: {ID: 6, Account: "legacy@example.com", PasswordEncrypted: "legacy-plain-password", MaintainedBy: 9},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	password, err := svc.RevealPassword(context.Background(), 6, &RevealWindsurfAccountPasswordInput{
		ActorID: 99,
		IsAdmin: true,
	})

	require.NoError(t, err)
	require.Equal(t, "legacy-plain-password", password)
	require.NotNil(t, repo.updated)
	require.Equal(t, "aesgcm:enc:legacy-plain-password", repo.updated.PasswordEncrypted)
}

func TestWindsurfAccountServiceRevealPasswordRejectsOpaqueBase64LikeValue(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			7: {ID: 7, Account: "legacy-base64@example.com", PasswordEncrypted: "gLIFwekrUh0I4cLLDxJRxk+vm8efpbxhLpcqj7mPqNAZHdz81sVtyw==", MaintainedBy: 9},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	password, err := svc.RevealPassword(context.Background(), 7, &RevealWindsurfAccountPasswordInput{
		ActorID: 99,
		IsAdmin: true,
	})

	require.ErrorIs(t, err, ErrWindsurfAccountPasswordUnreadable)
	require.Empty(t, password)
	require.Nil(t, repo.updated)
}

func TestWindsurfAccountServiceRevealPasswordReturnsActionableErrorForEmptyStoredPassword(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			8: {ID: 8, Account: "broken@example.com", PasswordEncrypted: "", MaintainedBy: 9},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	password, err := svc.RevealPassword(context.Background(), 8, &RevealWindsurfAccountPasswordInput{
		ActorID: 99,
		IsAdmin: true,
	})

	require.ErrorIs(t, err, ErrWindsurfAccountPasswordUnreadable)
	require.Empty(t, password)
	require.Nil(t, repo.updated)
}

func TestWindsurfAccountServiceDeleteRequiresAdmin(t *testing.T) {
	repo := &windsurfAccountRepoStub{}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	err := svc.Delete(context.Background(), 7, &DeleteWindsurfAccountInput{
		ActorID: 1,
		IsAdmin: false,
	})

	require.ErrorIs(t, err, ErrWindsurfAccountDeleteAdminOnly)
	require.Zero(t, repo.deletedID)
}

func TestWindsurfAccountServiceDeleteRemovesAccount(t *testing.T) {
	repo := &windsurfAccountRepoStub{
		byID: map[int64]*WindsurfAccount{
			7: {ID: 7, Account: "delete-me@example.com", PasswordEncrypted: "enc:secret"},
		},
	}
	svc := NewWindsurfAccountService(repo, &windsurfUserRepoStub{}, windsurfEncryptorStub{})

	err := svc.Delete(context.Background(), 7, &DeleteWindsurfAccountInput{
		ActorID: 1,
		IsAdmin: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(7), repo.deletedID)
}
