package accountquality

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCandyExplanationScoring(t *testing.T) {
	c := Condition{BenchmarkVersion: "2"}
	for _, tc := range []struct{ name, text, want string }{
		{"correct", `{"answer":21,"explanation":"## 解答\n选择数量如下。"}`, "passed"},
		{"order", `{"explanation":"答案说明","answer":21}`, "passed"},
		{"wrong answer ignores explanation", `{"answer":20,"explanation":"正确答案是21"}`, "incorrect"},
		{"missing explanation", `{"answer":21}`, "unknown"},
		{"missing answer", `{"explanation":"21"}`, "unknown"},
		{"duplicate answer", `{"answer":20,"answer":21,"explanation":"x"}`, "unknown"},
		{"duplicate explanation", `{"answer":21,"explanation":"x","explanation":"y"}`, "unknown"},
		{"extra", `{"answer":21,"explanation":"x","score":true}`, "unknown"},
		{"empty", `{"answer":21,"explanation":"  "}`, "unknown"},
		{"non-string", `{"answer":21,"explanation":{"text":"x"}}`, "unknown"},
		{"decimal", `{"answer":21.0,"explanation":"x"}`, "unknown"},
		{"exponent", `{"answer":21e0,"explanation":"x"}`, "unknown"},
		{"string answer", `{"answer":"21","explanation":"x"}`, "unknown"},
		{"truncated", `{"answer":21,"explanation":"x`, "unknown"},
		{"trailing", `{"answer":21,"explanation":"x"} prose`, "unknown"},
		{"fence", "```json\n{\"answer\":21,\"explanation\":\"x\"}\n```", "passed"},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, ScoreCandy(tc.text, true, c).Verdict) })
	}
	require.Equal(t, "incomplete_response", ScoreCandy(`{"answer":21,"explanation":"x"}`, false, c).ReasonCode)
	require.Equal(t, "response_too_large", ScoreCandy(strings.Repeat("x", MaxResponseBytes+1), true, c).ReasonCode)
	require.Equal(t, "unknown", Score(`{"answer":21,"explanation":"x"}`, true).Verdict)
}

func TestCandyExplanationVersionsAndExpiry(t *testing.T) {
	for _, suite := range []string{"candy", "dual"} {
		old := Condition{ModelID: "test", Protocol: "responses", Transport: "http", ReasoningEffort: "upstream_default", ServiceTier: "upstream_default", BenchmarkID: suite, BenchmarkVersion: "1", ScorerVersion: "strict-json-v1"}
		if suite == "dual" {
			old.ScorerVersion = "dual-visual-v1"
		}
		current := old
		current.BenchmarkVersion = "2"
		current.ScorerVersion = "answer-explanation-v2"
		if suite == "dual" {
			current.ScorerVersion = "dual-visual-v2"
		}
		require.NoError(t, old.Validate())
		require.NoError(t, current.Validate())
		require.NotContains(t, CandyPromptFor(old), "explanation")
		require.Contains(t, CandyPromptFor(current), "explanation")
		p, q := Policy{AccountID: 1}, Policy{AccountID: 1}
		p.SetTarget(Target{Condition: old})
		q.SetTarget(Target{Condition: current})
		require.NotEqual(t, p.ScopeHash, q.ScopeHash)
		require.Equal(t, p.ControlScopeHash, q.ControlScopeHash)
		invalid := current
		invalid.ScorerVersion = old.ScorerVersion
		require.Error(t, invalid.Validate())
		text := `{"answer":21,"explanation":"private solution"}`
		result := ScoreCandy(text, true, current)
		if suite == "dual" {
			result = Combine(Check(result), nil, nil, &Evidence{Candy: text})
		}
		raw, err := EncodeEvidence(result)
		require.NoError(t, err)
		metadata, err := json.Marshal(result)
		require.NoError(t, err)
		require.NotContains(t, string(metadata), "private solution")
		r := Run{Snapshot: q, Result: result, ResponseText: &raw}
		r.PresentEvidence(true)
		require.Equal(t, "private solution", PresentCandy(r).Explanation)
		r.PresentEvidence(false)
		require.Empty(t, PresentCandy(r).Explanation)
		require.Equal(t, CandyQuestion, PresentCandy(r).Question)
	}
}
