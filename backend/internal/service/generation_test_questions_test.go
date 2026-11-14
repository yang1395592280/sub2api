//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type questionTestRepo struct {
	PelicanTestRepository
	saved GenerationTestQuestion
	calls int
}

func (r *questionTestRepo) SaveQuestion(_ context.Context, q GenerationTestQuestion) (*GenerationTestQuestion, error) {
	r.saved = q
	r.calls++
	return &q, nil
}
func (r *questionTestRepo) ExpireStale(context.Context) error { return nil }
func (r *questionTestRepo) CreateQuestionBatch(_ context.Context, b string, a []int64, m, k string, q []int64, u int64) ([]PelicanTest, error) {
	r.calls++
	return []PelicanTest{{BatchID: b, QuestionID: q[0], AccountID: a[0]}}, nil
}
func TestGenerationQuestionValidation(t *testing.T) {
	repo := &questionTestRepo{}
	svc := &PelicanTestService{repo: repo}
	ctx := context.Background()
	q := GenerationTestQuestion{TestKind: TestKindIntelligence, Name: "  New question  ", Prompt: " Return a number ", ExpectedAnswer: "0042", Enabled: true}
	saved, err := svc.SaveQuestion(ctx, q)
	require.NoError(t, err)
	require.Equal(t, "42", saved.ExpectedAnswer)
	require.Equal(t, "New question", saved.Name)
	for _, answer := range []string{"", "-1", "42 apples", "1.5", strings.Repeat("9", 13)} {
		q.ExpectedAnswer = answer
		_, err = svc.SaveQuestion(ctx, q)
		require.ErrorIs(t, err, ErrInvalidPelicanBatch)
	}
	q.TestKind = TestKindPelican
	q.ExpectedAnswer = "secret answer"
	saved, err = svc.SaveQuestion(ctx, q)
	require.NoError(t, err)
	require.Empty(t, saved.ExpectedAnswer)
	for _, tc := range []struct {
		accounts, questions []int64
		model, kind         string
		user                int64
	}{
		{nil, []int64{1}, "text", "pelican", 1}, {[]int64{1, 1}, []int64{1}, "text", "pelican", 1},
		{[]int64{1}, []int64{1, 1}, "text", "pelican", 1}, {[]int64{1}, []int64{-1}, "text", "pelican", 1},
		{[]int64{1}, []int64{1}, "gpt-image-1", "pelican", 1}, {[]int64{1}, []int64{1}, "text", "bad", 1},
		{[]int64{1}, []int64{1}, "text", "pelican", 0},
	} {
		_, err = svc.StartQuestionBatch(ctx, tc.accounts, tc.model, tc.kind, tc.questions, tc.user)
		require.ErrorIs(t, err, ErrInvalidPelicanBatch)
	}
	accounts := make([]int64, 100)
	questions := make([]int64, 11)
	for i := range accounts {
		accounts[i] = int64(i + 1)
	}
	for i := range questions {
		questions[i] = int64(i + 1)
	}
	_, err = svc.StartQuestionBatch(ctx, accounts, "text", "pelican", questions, 1)
	require.ErrorIs(t, err, ErrInvalidPelicanBatch)
	rows, err := svc.StartQuestionBatch(ctx, accounts, "text", "pelican", questions[:10], 1)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
func TestGenerationQuestionScoresSnapshotWithoutLeakingAnswer(t *testing.T) {
	a := &Account{ID: 99, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Credentials: map[string]any{"api_key": "test", "base_url": "https://upstream.example/v1"}}
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"42\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{}}\n\n"
	tests, upstream := adaptiveCNAccountTestService(a, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))})
	svc := NewPelicanTestService(nil, tests.accountRepo, tests)
	prompt := "Solve this question. Return only the final number."
	status, _, _, _, _, _ := svc.generate(context.Background(), PelicanTest{TestKind: TestKindIntelligence, AccountID: a.ID, ModelID: "gpt-5.5", Prompt: prompt, ExpectedAnswer: "42"})
	require.Equal(t, "correct", status)
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, prompt, gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 1)
	require.NotContains(t, gjson.GetBytes(upstream.bodies[0], "input").Raw, "42")
	require.False(t, gjson.GetBytes(upstream.bodies[0], "expected_answer").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "tools").Exists())
}
