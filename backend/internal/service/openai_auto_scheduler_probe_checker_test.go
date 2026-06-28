package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type autoSchedulerProbeHTTPUpstreamStub struct {
	response *http.Response
	err      error
}

func (s *autoSchedulerProbeHTTPUpstreamStub) Do(*http.Request, string, int64, int) (*http.Response, error) {
	return nil, errors.New("unexpected Do call")
}

func (s *autoSchedulerProbeHTTPUpstreamStub) DoWithTLS(*http.Request, string, int64, int, *tlsfingerprint.Profile) (*http.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}

func TestOpenAIAutoSchedulerProbeHTTPCheckerRecordsLatencyForCompletedProbe(t *testing.T) {
	checker := NewOpenAIAutoSchedulerProbeChecker(&autoSchedulerProbeHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_probe"}`)),
		},
	}, nil)

	result := checker.Check(context.Background(), &Account{
		ID:          101,
		Platform:    PlatformOpenAI,
		Concurrency: 1,
		Credentials: map[string]any{"openai_api_key": "sk-test"},
	}, "gpt-5.4", time.Second)

	require.True(t, result.Success)
	require.NoError(t, result.Err)
	require.NotNil(t, result.LatencyMS)
	require.GreaterOrEqual(t, *result.LatencyMS, 0)
}
