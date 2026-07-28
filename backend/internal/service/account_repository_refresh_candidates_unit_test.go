//go:build unit

package service

import "context"

func (s *accountRepoStub) ListOAuthRefreshCandidates(context.Context) ([]Account, error) {
	panic("unexpected ListOAuthRefreshCandidates call")
}

func (s *accountRepoStub) ListSub2APICheckinCandidates(context.Context, int) ([]Account, error) {
	panic("unexpected ListSub2APICheckinCandidates call")
}

func (s *accountRepoStub) ListUpstreamBalanceRefreshCandidatesByGroupID(context.Context, int64, int) ([]Account, error) {
	panic("unexpected ListUpstreamBalanceRefreshCandidatesByGroupID call")
}

func (r *openAIAccountTestRepo) ListOAuthRefreshCandidates(context.Context) ([]Account, error) {
	panic("unexpected ListOAuthRefreshCandidates call")
}

func (r *openAIAccountTestRepo) ListSub2APICheckinCandidates(context.Context, int) ([]Account, error) {
	panic("unexpected ListSub2APICheckinCandidates call")
}

func (m *groupAwareMockAccountRepo) ListOAuthRefreshCandidates(context.Context) ([]Account, error) {
	panic("unexpected ListOAuthRefreshCandidates call")
}

func (m *groupAwareMockAccountRepo) ListSub2APICheckinCandidates(context.Context, int) ([]Account, error) {
	panic("unexpected ListSub2APICheckinCandidates call")
}

func (m *mockAccountRepoForPlatform) ListOAuthRefreshCandidates(context.Context) ([]Account, error) {
	panic("unexpected ListOAuthRefreshCandidates call")
}

func (m *mockAccountRepoForPlatform) ListSub2APICheckinCandidates(context.Context, int) ([]Account, error) {
	panic("unexpected ListSub2APICheckinCandidates call")
}

func (m *mockAccountRepoForPlatform) ListUpstreamBalanceRefreshCandidatesByGroupID(context.Context, int64, int) ([]Account, error) {
	panic("unexpected ListUpstreamBalanceRefreshCandidatesByGroupID call")
}

func (m *mockAccountRepoForGemini) ListOAuthRefreshCandidates(context.Context) ([]Account, error) {
	return m.ListActive(context.Background())
}

func (m *mockAccountRepoForGemini) ListSub2APICheckinCandidates(context.Context, int) ([]Account, error) {
	panic("unexpected ListSub2APICheckinCandidates call")
}

func (m *mockAccountRepoForGemini) ListUpstreamBalanceRefreshCandidatesByGroupID(context.Context, int64, int) ([]Account, error) {
	panic("unexpected ListUpstreamBalanceRefreshCandidatesByGroupID call")
}
