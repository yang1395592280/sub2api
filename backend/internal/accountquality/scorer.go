package accountquality

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

// The problem layout comes from manxue-ai server.py at revision
// 5da79674417dd0748682f09b7b7467efd1968ea8 (Apache-2.0).
// Adaptation: specify the shape-selection premise and require one final JSON.
// Attribution and license: third_party/manxue-ai/{NOTICE,upstream/LICENSE}.
const CandyPrompt = `袋中有两种形状、三种口味的糖果：
圆形：苹果味7颗、桃子味9颗、西瓜味8颗。
五角星：苹果味7颗、桃子味6颗、西瓜味4颗。
你能凭手感选择形状，但无法识别口味。你必须事先固定取多少颗圆形、多少颗五角星，不能根据抽出的口味调整；同一形状内可能取到任意组合。
至少取多少颗，才能在最不利情况下仍保证取到以下至少一种搭配：一颗圆形苹果味与一颗五角星桃子味，或一颗圆形桃子味与一颗五角星苹果味？
只输出一个JSON对象，唯一字段answer的值为最小总颗数的整数，不输出其他正文。`

// Score rejects ambiguous output, duplicate keys and incomplete generations.
// It never searches reasoning or prose for the expected number.
func Score(text string, completed bool) Result {
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
	value := strings.TrimSpace(text)
	if strings.HasPrefix(value, "```") {
		newline := strings.IndexByte(value, '\n')
		if newline < 0 || !strings.HasSuffix(value, "```") {
			return r
		}
		language := strings.TrimSpace(value[3:newline])
		if language != "" && language != "json" {
			return r
		}
		value = strings.TrimSpace(value[newline+1 : len(value)-3])
	}
	d := json.NewDecoder(strings.NewReader(value))
	d.UseNumber()
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return r
	}
	if !d.More() {
		return r
	}
	key, err := d.Token()
	if err != nil || key != "answer" {
		return r
	}
	raw, err := d.Token()
	if err != nil {
		return r
	}
	answer, ok := raw.(json.Number)
	if !ok {
		return r
	}
	if strings.ContainsAny(answer.String(), ".eE") {
		return r
	}
	n, err := strconv.ParseInt(answer.String(), 10, 64)
	if err != nil {
		return r
	}
	if d.More() {
		return r
	}
	t, err = d.Token()
	if err != nil || t != json.Delim('}') {
		return r
	}
	if _, err = d.Token(); err != io.EOF {
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
