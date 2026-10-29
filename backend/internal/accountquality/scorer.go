package accountquality

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

// The problem layout comes from manxue-ai server.py at revision
// 5da79674417dd0748682f09b7b7467efd1968ea8 (Apache-2.0).
// Adaptation: specify shape selection and version the final answer/summary format.
// Attribution and license: third_party/manxue-ai/{NOTICE,upstream/LICENSE}.
const CandyQuestion = `袋中有两种形状、三种口味的糖果：
圆形：苹果味7颗、桃子味9颗、西瓜味8颗。
五角星：苹果味7颗、桃子味6颗、西瓜味4颗。
你能凭手感选择形状，但无法识别口味。你必须事先固定取多少颗圆形、多少颗五角星，不能根据抽出的口味调整；同一形状内可能取到任意组合。
至少取多少颗，才能在最不利情况下仍保证取到以下至少一种搭配：一颗圆形苹果味与一颗五角星桃子味，或一颗圆形桃子味与一颗五角星苹果味？`

const CandyPrompt = CandyQuestion + "\n只输出一个JSON对象，唯一字段answer的值为最小总颗数的整数，不输出其他正文。"
const CandyExplanationPrompt = CandyQuestion + `
只输出一个JSON对象，恰好包含answer和explanation两个字段。answer为最小总颗数的整数；explanation为面向读者的解题说明字符串，可用Markdown标题、列表和表格排版。
说明应概述所选两种形状的数量、为何保证成功、为何更少不行，保持简明且可核对。不输出内部思考过程，不在JSON之外输出正文。`

func CandyPromptFor(c Condition) string {
	if c.BenchmarkVersion == "2" {
		return CandyExplanationPrompt
	}
	return CandyPrompt
}

func ScoreCandy(text string, completed bool, c Condition) Result {
	return scoreCandy(text, completed, c.BenchmarkVersion == "2")
}

// Score rejects ambiguous output, duplicate keys and incomplete generations.
// It never searches reasoning or prose for the expected number.
func Score(text string, completed bool) Result {
	return scoreCandy(text, completed, false)
}

func scoreCandy(text string, completed, withExplanation bool) Result {
	r := Result{Verdict: "unknown", ReasonCode: "invalid_answer_format", ExpectedAnswer: 21, ResponseText: text}
	if len(text) > MaxResponseBytes {
		r.ResponseTruncated = true
		r.ResponseText = strings.ToValidUTF8(text[:MaxResponseBytes], "")
		r.ReasonCode = "response_too_large"
		return r
	}
	if !completed {
		r.ReasonCode = "incomplete_response"
		return r
	}
	n, _, ok := parseCandyAnswer(text, withExplanation)
	if !ok {
		return r
	}
	r.FinalAnswer = &n
	r.Verdict = "incorrect"
	r.ReasonCode = "wrong_answer"
	if n == 21 {
		r.Verdict = "passed"
		r.ReasonCode = "correct_answer"
	}
	return r
}

// Token parsing rejects duplicate keys, coercion and trailing prose for both versions.
func parseCandyAnswer(text string, withExplanation bool) (int64, string, bool) {
	if len(text) > MaxResponseBytes {
		return 0, "", false
	}
	value := strings.TrimSpace(text)
	if strings.HasPrefix(value, "```") {
		newline := strings.IndexByte(value, '\n')
		if newline < 0 || !strings.HasSuffix(value, "```") {
			return 0, "", false
		}
		language := strings.TrimSpace(value[3:newline])
		if language != "" && language != "json" {
			return 0, "", false
		}
		value = strings.TrimSpace(value[newline+1 : len(value)-3])
	}
	d := json.NewDecoder(strings.NewReader(value))
	d.UseNumber()
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return 0, "", false
	}
	seen := map[string]bool{}
	var n int64
	var explanation string
	for d.More() {
		key, err := d.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[name] {
			return 0, "", false
		}
		seen[name] = true
		raw, err := d.Token()
		if err != nil {
			return 0, "", false
		}
		switch name {
		case "answer":
			answer, ok := raw.(json.Number)
			if !ok || strings.ContainsAny(answer.String(), ".eE") {
				return 0, "", false
			}
			n, err = strconv.ParseInt(answer.String(), 10, 64)
			if err != nil {
				return 0, "", false
			}
		case "explanation":
			var ok bool
			explanation, ok = raw.(string)
			if !withExplanation || !ok || strings.TrimSpace(explanation) == "" {
				return 0, "", false
			}
		default:
			return 0, "", false
		}
	}
	t, err = d.Token()
	if err != nil || t != json.Delim('}') || !seen["answer"] || (withExplanation && !seen["explanation"]) {
		return 0, "", false
	}
	if _, err = d.Token(); err != io.EOF {
		return 0, "", false
	}
	return n, explanation, true
}

type CandyDetail struct {
	Question             string `json:"question"`
	Prompt               string `json:"prompt"`
	ExplanationRequested bool   `json:"explanation_requested"`
	Explanation          string `json:"explanation,omitempty"`
}

// Called only after PresentEvidence has removed omitted or expired response bodies.
func PresentCandy(r Run) CandyDetail {
	c := r.Snapshot.Condition
	detail := CandyDetail{Question: CandyQuestion, Prompt: CandyPromptFor(c), ExplanationRequested: c.BenchmarkVersion == "2"}
	var text string
	if r.Evidence != nil {
		text = r.Evidence.Candy
	} else if r.ResponseText != nil {
		text = *r.ResponseText
	}
	if detail.ExplanationRequested {
		_, detail.Explanation, _ = parseCandyAnswer(text, true)
	}
	return detail
}
