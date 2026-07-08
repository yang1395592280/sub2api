package service

import (
	"context"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// GroupCapacitySummary holds aggregated capacity for a single group.
type GroupCapacitySummary struct {
	GroupID         int64 `json:"group_id"`
	ConcurrencyUsed int   `json:"concurrency_used"`
	ConcurrencyMax  int   `json:"concurrency_max"`
	SessionsUsed    int   `json:"sessions_used"`
	SessionsMax     int   `json:"sessions_max"`
	RPMUsed         int   `json:"rpm_used"`
	RPMMax          int   `json:"rpm_max"`
}

// GroupCapacityUserDetail holds per-user runtime and limit details for a group.
type GroupCapacityUserDetail struct {
	UserID             int64  `json:"user_id"`
	Username           string `json:"username"`
	Email              string `json:"email"`
	Notes              string `json:"notes"`
	Status             string `json:"status"`
	CurrentConcurrency int    `json:"current_concurrency"`
	ConcurrencyLimit   int    `json:"concurrency_limit"`
	CurrentRPM         int    `json:"current_rpm"`
	EffectiveRPMLimit  int    `json:"effective_rpm_limit"`
	RPMLimitSource     string `json:"rpm_limit_source"`
	RPMOverride        *int   `json:"rpm_override,omitempty"`
	GroupRPMLimit      int    `json:"group_rpm_limit"`
	UserRPMLimit       int    `json:"user_rpm_limit"`
}

// GroupAccountCapacityRow is the lightweight account projection needed for
// capacity summary aggregation.
type GroupAccountCapacityRow struct {
	GroupID             int64
	AccountID           int64
	Concurrency         int
	Extra               map[string]any
	SessionWindowStart  *time.Time
	SessionWindowEnd    *time.Time
	SessionWindowStatus string
}

type groupCapacityActiveGroupIDLister interface {
	ListActiveIDs(ctx context.Context) ([]int64, error)
}

type groupCapacityAccountLister interface {
	ListSchedulableCapacityByGroupIDs(ctx context.Context, groupIDs []int64) ([]GroupAccountCapacityRow, error)
}

// GroupCapacityService aggregates per-group capacity from runtime data.
type GroupCapacityService struct {
	accountRepo        AccountRepository
	groupRepo          GroupRepository
	concurrencyService *ConcurrencyService
	sessionLimitCache  SessionLimitCache
	rpmCache           RPMCache
	userRepo           UserRepository
	userSubRepo        UserSubscriptionRepository
	userGroupRateRepo  UserGroupRateRepository
	userRPMCache       UserRPMCache
}

// NewGroupCapacityService creates a new GroupCapacityService.
func NewGroupCapacityService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	concurrencyService *ConcurrencyService,
	sessionLimitCache SessionLimitCache,
	rpmCache RPMCache,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	userGroupRateRepo UserGroupRateRepository,
	userRPMCache UserRPMCache,
) *GroupCapacityService {
	return &GroupCapacityService{
		accountRepo:        accountRepo,
		groupRepo:          groupRepo,
		concurrencyService: concurrencyService,
		sessionLimitCache:  sessionLimitCache,
		rpmCache:           rpmCache,
		userRepo:           userRepo,
		userSubRepo:        userSubRepo,
		userGroupRateRepo:  userGroupRateRepo,
		userRPMCache:       userRPMCache,
	}
}

// GetAllGroupCapacity returns capacity summary for all active groups.
func (s *GroupCapacityService) GetAllGroupCapacity(ctx context.Context) ([]GroupCapacitySummary, error) {
	groupIDs, err := s.listActiveGroupIDs(ctx)
	if err != nil {
		return nil, err
	}

	if lister, ok := s.accountRepo.(groupCapacityAccountLister); ok {
		return s.getGroupCapacitiesBatch(ctx, groupIDs, lister)
	}

	return s.getGroupCapacitiesSequential(ctx, groupIDs), nil
}

func (s *GroupCapacityService) listActiveGroupIDs(ctx context.Context) ([]int64, error) {
	if lister, ok := s.groupRepo.(groupCapacityActiveGroupIDLister); ok {
		return lister.ListActiveIDs(ctx)
	}

	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]int64, 0, len(groups))
	for i := range groups {
		groupIDs = append(groupIDs, groups[i].ID)
	}
	return groupIDs, nil
}

func (s *GroupCapacityService) getGroupCapacitiesSequential(ctx context.Context, groupIDs []int64) []GroupCapacitySummary {
	results := make([]GroupCapacitySummary, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		cap, err := s.getGroupCapacity(ctx, groupID)
		if err != nil {
			// Skip groups with errors, return partial results
			continue
		}
		cap.GroupID = groupID
		results = append(results, cap)
	}
	return results
}

type groupCapacityAccountRef struct {
	groupID   int64
	accountID int64
}

func (s *GroupCapacityService) getGroupCapacitiesBatch(ctx context.Context, groupIDs []int64, lister groupCapacityAccountLister) ([]GroupCapacitySummary, error) {
	results := make([]GroupCapacitySummary, len(groupIDs))
	groupIndex := make(map[int64]int, len(groupIDs))
	for i, groupID := range groupIDs {
		results[i].GroupID = groupID
		groupIndex[groupID] = i
	}
	if len(groupIDs) == 0 {
		return results, nil
	}

	rows, err := lister.ListSchedulableCapacityByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return results, nil
	}

	refs := make([]groupCapacityAccountRef, 0, len(rows))
	seenGroupAccount := make(map[groupCapacityAccountRef]struct{}, len(rows))
	accountIDSet := make(map[int64]struct{}, len(rows))
	accountIDs := make([]int64, 0, len(rows))
	sessionTimeouts := make(map[int64]time.Duration)

	for _, row := range rows {
		idx, ok := groupIndex[row.GroupID]
		if !ok || row.AccountID <= 0 {
			continue
		}

		ref := groupCapacityAccountRef{groupID: row.GroupID, accountID: row.AccountID}
		if _, ok := seenGroupAccount[ref]; ok {
			continue
		}
		seenGroupAccount[ref] = struct{}{}
		refs = append(refs, ref)

		if _, ok := accountIDSet[row.AccountID]; !ok {
			accountIDSet[row.AccountID] = struct{}{}
			accountIDs = append(accountIDs, row.AccountID)
		}

		acc := Account{
			ID:                  row.AccountID,
			Concurrency:         row.Concurrency,
			Extra:               row.Extra,
			SessionWindowStart:  row.SessionWindowStart,
			SessionWindowEnd:    row.SessionWindowEnd,
			SessionWindowStatus: row.SessionWindowStatus,
		}

		results[idx].ConcurrencyMax += acc.Concurrency

		if maxSessions := acc.GetMaxSessions(); maxSessions > 0 {
			results[idx].SessionsMax += maxSessions
			timeout := time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			if timeout <= 0 {
				timeout = 5 * time.Minute
			}
			sessionTimeouts[acc.ID] = timeout
		}

		if rpm := acc.GetBaseRPM(); rpm > 0 {
			results[idx].RPMMax += rpm
		}
	}

	if len(accountIDs) == 0 {
		return results, nil
	}

	concurrencyMap := map[int64]int{}
	if s.concurrencyService != nil {
		concurrencyMap, _ = s.concurrencyService.GetAccountConcurrencyBatch(ctx, accountIDs)
	}

	sessionAccountIDs := accountIDsForGroupsWithLimit(refs, groupIndex, results, func(summary GroupCapacitySummary) bool {
		return summary.SessionsMax > 0
	})
	var sessionsMap map[int64]int
	if len(sessionAccountIDs) > 0 && s.sessionLimitCache != nil {
		sessionsMap, _ = s.sessionLimitCache.GetActiveSessionCountBatch(ctx, sessionAccountIDs, sessionTimeouts)
	}

	rpmAccountIDs := accountIDsForGroupsWithLimit(refs, groupIndex, results, func(summary GroupCapacitySummary) bool {
		return summary.RPMMax > 0
	})
	var rpmMap map[int64]int
	if len(rpmAccountIDs) > 0 && s.rpmCache != nil {
		rpmMap, _ = s.rpmCache.GetRPMBatch(ctx, rpmAccountIDs)
	}

	for _, ref := range refs {
		idx := groupIndex[ref.groupID]
		results[idx].ConcurrencyUsed += concurrencyMap[ref.accountID]
		if sessionsMap != nil && results[idx].SessionsMax > 0 {
			results[idx].SessionsUsed += sessionsMap[ref.accountID]
		}
		if rpmMap != nil && results[idx].RPMMax > 0 {
			results[idx].RPMUsed += rpmMap[ref.accountID]
		}
	}
	return results, nil
}

// GetGroupCapacityUsers returns paginated per-user capacity details for a group.
func (s *GroupCapacityService) GetGroupCapacityUsers(ctx context.Context, groupID int64, params pagination.PaginationParams, activeOnly bool) ([]GroupCapacityUserDetail, int64, error) {
	if s == nil || s.groupRepo == nil || s.userRepo == nil {
		return []GroupCapacityUserDetail{}, 0, nil
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}

	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return nil, 0, err
	}
	users, err := s.listGroupCapacityCandidateUsers(ctx, group)
	if err != nil {
		return nil, 0, err
	}
	if len(users) == 0 {
		return []GroupCapacityUserDetail{}, 0, nil
	}

	userIDs := make([]int64, 0, len(users))
	userLoadReq := make([]UserWithConcurrency, 0, len(users))
	for i := range users {
		if users[i].ID <= 0 {
			continue
		}
		userIDs = append(userIDs, users[i].ID)
		userLoadReq = append(userLoadReq, UserWithConcurrency{ID: users[i].ID, MaxConcurrency: users[i].Concurrency})
	}
	concurrencyMap := map[int64]int{}
	if s.concurrencyService != nil && len(userLoadReq) > 0 {
		if _, ok := s.concurrencyService.cache.(UserGroupConcurrencyReader); ok {
			groupLoads, loadErr := s.concurrencyService.GetUserGroupConcurrencyBatch(ctx, group.ID, userIDs)
			if loadErr == nil && groupLoads != nil {
				concurrencyMap = groupLoads
			}
		}
		if len(concurrencyMap) == 0 {
			if loads, loadErr := s.concurrencyService.GetUsersLoadBatch(ctx, userLoadReq); loadErr == nil && loads != nil {
				for userID, load := range loads {
					if load != nil {
						concurrencyMap[userID] = load.CurrentConcurrency
					}
				}
			}
		}
	}

	items := make([]GroupCapacityUserDetail, 0, len(users))
	for i := range users {
		user := users[i]
		if user.ID <= 0 {
			continue
		}
		item := GroupCapacityUserDetail{
			UserID:           user.ID,
			Username:         user.Username,
			Email:            user.Email,
			Notes:            user.Notes,
			Status:           user.Status,
			ConcurrencyLimit: user.Concurrency,
			GroupRPMLimit:    group.RPMLimit,
			UserRPMLimit:     user.RPMLimit,
		}
		item.CurrentConcurrency = concurrencyMap[user.ID]
		item.RPMOverride, item.EffectiveRPMLimit, item.RPMLimitSource, item.CurrentRPM = s.resolveGroupCapacityUserRPM(ctx, user.ID, group.ID, user.RPMLimit, group.RPMLimit)
		if activeOnly && item.CurrentConcurrency == 0 && item.CurrentRPM == 0 {
			continue
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CurrentConcurrency != items[j].CurrentConcurrency {
			return items[i].CurrentConcurrency > items[j].CurrentConcurrency
		}
		if items[i].CurrentRPM != items[j].CurrentRPM {
			return items[i].CurrentRPM > items[j].CurrentRPM
		}
		return items[i].UserID < items[j].UserID
	})

	total := int64(len(items))
	start := (params.Page - 1) * params.PageSize
	if start >= len(items) {
		return []GroupCapacityUserDetail{}, total, nil
	}
	end := start + params.PageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func (s *GroupCapacityService) listGroupCapacityCandidateUsers(ctx context.Context, group *Group) ([]User, error) {
	if group == nil || s.userRepo == nil {
		return []User{}, nil
	}
	if group.IsSubscriptionType() {
		return s.listSubscribedGroupUsers(ctx, group.ID)
	}
	if group.IsExclusive {
		return s.userRepo.ListAllowedUsersByGroupID(ctx, group.ID)
	}
	return s.listAllActiveUsers(ctx)
}

func (s *GroupCapacityService) listSubscribedGroupUsers(ctx context.Context, groupID int64) ([]User, error) {
	if s.userSubRepo == nil {
		return []User{}, nil
	}
	users := make([]User, 0)
	seen := make(map[int64]struct{})
	for page := 1; ; page++ {
		subs, result, err := s.userSubRepo.List(
			ctx,
			pagination.PaginationParams{Page: page, PageSize: 200, SortBy: "created_at", SortOrder: "DESC"},
			nil,
			&groupID,
			SubscriptionStatusActive,
			"",
			"created_at",
			"DESC",
		)
		if err != nil {
			return nil, err
		}
		for i := range subs {
			if subs[i].User == nil || subs[i].User.ID <= 0 {
				continue
			}
			if _, ok := seen[subs[i].User.ID]; ok {
				continue
			}
			seen[subs[i].User.ID] = struct{}{}
			users = append(users, *subs[i].User)
		}
		if len(subs) == 0 || result == nil || int64(len(users)) >= result.Total {
			break
		}
	}
	return users, nil
}

func (s *GroupCapacityService) listAllActiveUsers(ctx context.Context) ([]User, error) {
	users := make([]User, 0)
	includeSubscriptions := false
	for page := 1; ; page++ {
		batch, result, err := s.userRepo.ListWithFilters(
			ctx,
			pagination.PaginationParams{Page: page, PageSize: 500, SortBy: "id", SortOrder: "asc"},
			UserListFilters{Status: StatusActive, IncludeSubscriptions: &includeSubscriptions},
		)
		if err != nil {
			return nil, err
		}
		users = append(users, batch...)
		if len(batch) == 0 || result == nil || int64(len(users)) >= result.Total {
			break
		}
	}
	return users, nil
}

func (s *GroupCapacityService) resolveGroupCapacityUserRPM(ctx context.Context, userID, groupID int64, userRPMLimit, groupRPMLimit int) (*int, int, string, int) {
	var override *int
	if s.userGroupRateRepo != nil {
		if value, err := s.userGroupRateRepo.GetRPMOverrideByUserAndGroup(ctx, userID, groupID); err == nil {
			override = value
		}
	}

	if override != nil {
		return override, *override, "override", s.getUserGroupRPMBestEffort(ctx, userID, groupID)
	}
	if groupRPMLimit > 0 {
		return nil, groupRPMLimit, "group", s.getUserGroupRPMBestEffort(ctx, userID, groupID)
	}
	if userRPMLimit > 0 {
		return nil, userRPMLimit, "user", s.getUserRPMBestEffort(ctx, userID)
	}
	return nil, 0, "unlimited", 0
}

func (s *GroupCapacityService) getUserGroupRPMBestEffort(ctx context.Context, userID, groupID int64) int {
	if s.userRPMCache == nil {
		return 0
	}
	count, err := s.userRPMCache.GetUserGroupRPM(ctx, userID, groupID)
	if err != nil {
		return 0
	}
	return count
}

func (s *GroupCapacityService) getUserRPMBestEffort(ctx context.Context, userID int64) int {
	if s.userRPMCache == nil {
		return 0
	}
	count, err := s.userRPMCache.GetUserRPM(ctx, userID)
	if err != nil {
		return 0
	}
	return count
}

func accountIDsForGroupsWithLimit(refs []groupCapacityAccountRef, groupIndex map[int64]int, summaries []GroupCapacitySummary, include func(GroupCapacitySummary) bool) []int64 {
	seen := make(map[int64]struct{})
	accountIDs := make([]int64, 0)
	for _, ref := range refs {
		idx, ok := groupIndex[ref.groupID]
		if !ok || !include(summaries[idx]) {
			continue
		}
		if _, ok := seen[ref.accountID]; ok {
			continue
		}
		seen[ref.accountID] = struct{}{}
		accountIDs = append(accountIDs, ref.accountID)
	}
	return accountIDs
}

func (s *GroupCapacityService) getGroupCapacity(ctx context.Context, groupID int64) (GroupCapacitySummary, error) {
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, groupID)
	if err != nil {
		return GroupCapacitySummary{}, err
	}
	if len(accounts) == 0 {
		return GroupCapacitySummary{}, nil
	}

	// Collect account IDs and config values
	accountIDs := make([]int64, 0, len(accounts))
	sessionTimeouts := make(map[int64]time.Duration)
	var concurrencyMax, sessionsMax, rpmMax int

	for i := range accounts {
		acc := &accounts[i]
		accountIDs = append(accountIDs, acc.ID)
		concurrencyMax += acc.Concurrency

		if ms := acc.GetMaxSessions(); ms > 0 {
			sessionsMax += ms
			timeout := time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			if timeout <= 0 {
				timeout = 5 * time.Minute
			}
			sessionTimeouts[acc.ID] = timeout
		}

		if rpm := acc.GetBaseRPM(); rpm > 0 {
			rpmMax += rpm
		}
	}

	// Batch query runtime data from Redis
	concurrencyMap, _ := s.concurrencyService.GetAccountConcurrencyBatch(ctx, accountIDs)

	var sessionsMap map[int64]int
	if sessionsMax > 0 && s.sessionLimitCache != nil {
		sessionsMap, _ = s.sessionLimitCache.GetActiveSessionCountBatch(ctx, accountIDs, sessionTimeouts)
	}

	var rpmMap map[int64]int
	if rpmMax > 0 && s.rpmCache != nil {
		rpmMap, _ = s.rpmCache.GetRPMBatch(ctx, accountIDs)
	}

	// Aggregate
	var concurrencyUsed, sessionsUsed, rpmUsed int
	for _, id := range accountIDs {
		concurrencyUsed += concurrencyMap[id]
		if sessionsMap != nil {
			sessionsUsed += sessionsMap[id]
		}
		if rpmMap != nil {
			rpmUsed += rpmMap[id]
		}
	}

	return GroupCapacitySummary{
		ConcurrencyUsed: concurrencyUsed,
		ConcurrencyMax:  concurrencyMax,
		SessionsUsed:    sessionsUsed,
		SessionsMax:     sessionsMax,
		RPMUsed:         rpmUsed,
		RPMMax:          rpmMax,
	}, nil
}
