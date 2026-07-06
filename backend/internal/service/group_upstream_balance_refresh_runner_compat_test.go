package service

import "context"

func (m *sessionWindowMockRepo) ListUpstreamBalanceRefreshCandidatesByGroupID(context.Context, int64, int) ([]Account, error) {
	panic("unexpected ListUpstreamBalanceRefreshCandidatesByGroupID call")
}

func (s *stubAntigravityAccountRepo) ListUpstreamBalanceRefreshCandidatesByGroupID(context.Context, int64, int) ([]Account, error) {
	panic("unexpected ListUpstreamBalanceRefreshCandidatesByGroupID call")
}

func (r stubOpenAIAccountRepo) ListUpstreamBalanceRefreshCandidatesByGroupID(context.Context, int64, int) ([]Account, error) {
	panic("unexpected ListUpstreamBalanceRefreshCandidatesByGroupID call")
}

func (s *apiKeyServiceGroupRepoStub) ListUpstreamBalanceRefreshEnabled(context.Context) ([]Group, error) {
	panic("unexpected ListUpstreamBalanceRefreshEnabled call")
}

func (groupRepoNoop) ListUpstreamBalanceRefreshEnabled(context.Context) ([]Group, error) {
	panic("unexpected ListUpstreamBalanceRefreshEnabled call")
}
