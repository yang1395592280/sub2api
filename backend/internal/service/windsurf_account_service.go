package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type WindsurfAccountService struct {
	repo      WindsurfAccountRepository
	userRepo  UserRepository
	encryptor SecretEncryptor
}

const windsurfEncryptedPasswordPrefix = "aesgcm:"
const windsurfLegacyCiphertextMinBytes = 12 + 16

func NewWindsurfAccountService(
	repo WindsurfAccountRepository,
	userRepo UserRepository,
	encryptor SecretEncryptor,
) *WindsurfAccountService {
	return &WindsurfAccountService{
		repo:      repo,
		userRepo:  userRepo,
		encryptor: encryptor,
	}
}

func (s *WindsurfAccountService) List(
	ctx context.Context,
	params pagination.PaginationParams,
	filters WindsurfAccountListFilters,
) ([]WindsurfAccountListItem, *pagination.PaginationResult, error) {
	items, result, err := s.repo.List(ctx, params, WindsurfAccountListFilters{
		Search: strings.TrimSpace(filters.Search),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list windsurf accounts: %w", err)
	}

	out := make([]WindsurfAccountListItem, 0, len(items))
	for i := range items {
		out = append(out, s.toListItem(ctx, &items[i]))
	}
	return out, result, nil
}

func (s *WindsurfAccountService) Create(ctx context.Context, input *CreateWindsurfAccountInput) (*WindsurfAccountListItem, error) {
	if input == nil {
		return nil, ErrWindsurfAccountAccountRequired
	}

	account := normalizeWindsurfAccountValue(input.Account)
	if account == "" {
		return nil, ErrWindsurfAccountAccountRequired
	}
	if strings.TrimSpace(input.Password) == "" {
		return nil, ErrWindsurfAccountPasswordRequired
	}

	encrypted, err := s.encryptWindsurfPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("encrypt windsurf password: %w", err)
	}

	now := time.Now()
	record := &WindsurfAccount{
		Account:           account,
		PasswordEncrypted: encrypted,
		Enabled:           false,
		MaintainedBy:      input.ActorID,
		MaintainedAt:      now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("create windsurf account: %w", err)
	}

	item := s.toListItem(ctx, record)
	return &item, nil
}

func (s *WindsurfAccountService) UpdateCredentials(ctx context.Context, id int64, input *UpdateWindsurfAccountCredentialsInput) (*WindsurfAccountListItem, error) {
	if input == nil {
		return nil, ErrWindsurfAccountAccountRequired
	}

	record, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !input.IsAdmin && input.ActorID != record.MaintainedBy {
		return nil, ErrWindsurfAccountUpdateDenied
	}

	if input.IsAdmin {
		account := normalizeWindsurfAccountValue(input.Account)
		if account == "" {
			return nil, ErrWindsurfAccountAccountRequired
		}
		record.Account = account
	}

	password := strings.TrimSpace(input.Password)
	if password != "" {
		encrypted, err := s.encryptWindsurfPassword(input.Password)
		if err != nil {
			return nil, fmt.Errorf("encrypt windsurf password: %w", err)
		}
		record.PasswordEncrypted = encrypted
	} else if !input.IsAdmin {
		return nil, ErrWindsurfAccountPasswordRequired
	}

	now := time.Now()
	record.Enabled = false
	record.MaintainedBy = input.ActorID
	record.MaintainedAt = now
	record.StatusUpdatedBy = nil
	record.StatusUpdatedAt = nil
	record.UpdatedAt = now

	if err := s.repo.Update(ctx, record); err != nil {
		return nil, fmt.Errorf("update windsurf account credentials: %w", err)
	}

	item := s.toListItem(ctx, record)
	return &item, nil
}

func (s *WindsurfAccountService) UpdateStatus(ctx context.Context, id int64, input *UpdateWindsurfAccountStatusInput) (*WindsurfAccountListItem, error) {
	if input == nil || !input.IsAdmin {
		return nil, ErrWindsurfAccountStatusAdminOnly
	}

	record, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	record.Enabled = input.Enabled
	record.StatusUpdatedBy = &input.ActorID
	record.StatusUpdatedAt = &now
	record.UpdatedAt = now

	if err := s.repo.Update(ctx, record); err != nil {
		return nil, fmt.Errorf("update windsurf account status: %w", err)
	}

	item := s.toListItem(ctx, record)
	return &item, nil
}

func (s *WindsurfAccountService) RevealPassword(ctx context.Context, id int64, input *RevealWindsurfAccountPasswordInput) (string, error) {
	record, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if input == nil || (!input.IsAdmin && input.ActorID != record.MaintainedBy) {
		return "", ErrWindsurfAccountPasswordViewDenied
	}

	password, migratedEncrypted, err := s.resolveStoredPassword(record.PasswordEncrypted)
	if err != nil {
		return "", err
	}
	if migratedEncrypted != "" {
		record.PasswordEncrypted = migratedEncrypted
		if err := s.repo.Update(ctx, record); err != nil {
			return "", fmt.Errorf("migrate legacy windsurf password: %w", err)
		}
	}
	return password, nil
}

func (s *WindsurfAccountService) Delete(ctx context.Context, id int64, input *DeleteWindsurfAccountInput) error {
	if input == nil || !input.IsAdmin {
		return ErrWindsurfAccountDeleteAdminOnly
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete windsurf account: %w", err)
	}
	return nil
}

func (s *WindsurfAccountService) toListItem(ctx context.Context, record *WindsurfAccount) WindsurfAccountListItem {
	item := WindsurfAccountListItem{
		ID:              record.ID,
		Account:         record.Account,
		PasswordMasked:  maskWindsurfPassword(record.PasswordEncrypted),
		Enabled:         record.Enabled,
		MaintainedByID:  record.MaintainedBy,
		MaintainedAt:    record.MaintainedAt,
		StatusUpdatedBy: record.StatusUpdatedBy,
		StatusUpdatedAt: record.StatusUpdatedAt,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}

	if s.userRepo == nil || record.MaintainedBy <= 0 {
		return item
	}
	user, err := s.userRepo.GetByID(ctx, record.MaintainedBy)
	if err != nil || user == nil {
		return item
	}
	item.MaintainedByName = user.Username
	item.MaintainedByEmail = user.Email
	return item
}

func (s *WindsurfAccountService) resolveStoredPassword(stored string) (password string, migratedEncrypted string, err error) {
	if strings.TrimSpace(stored) == "" {
		return "", "", ErrWindsurfAccountPasswordUnreadable
	}

	if encrypted, ok := strings.CutPrefix(stored, windsurfEncryptedPasswordPrefix); ok {
		password, err = s.encryptor.Decrypt(encrypted)
		if err != nil {
			return "", "", ErrWindsurfAccountPasswordUnreadable
		}
		return password, "", nil
	}

	password, err = s.encryptor.Decrypt(stored)
	if err == nil {
		encrypted, encryptErr := s.encryptWindsurfPassword(password)
		if encryptErr != nil {
			return "", "", fmt.Errorf("normalize legacy encrypted windsurf password: %w", encryptErr)
		}
		return password, encrypted, nil
	}
	if looksLikeUnreadableLegacyWindsurfCiphertext(stored) {
		return "", "", ErrWindsurfAccountPasswordUnreadable
	}
	encrypted, encryptErr := s.encryptWindsurfPassword(stored)
	if encryptErr != nil {
		return "", "", fmt.Errorf("encrypt legacy windsurf password: %w", encryptErr)
	}
	return stored, encrypted, nil
}

func (s *WindsurfAccountService) encryptWindsurfPassword(password string) (string, error) {
	encrypted, err := s.encryptor.Encrypt(password)
	if err != nil {
		return "", err
	}
	return windsurfEncryptedPasswordPrefix + encrypted, nil
}

func looksLikeUnreadableLegacyWindsurfCiphertext(stored string) bool {
	decoded, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		return false
	}
	return len(decoded) >= windsurfLegacyCiphertextMinBytes
}
