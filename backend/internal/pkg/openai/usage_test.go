package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCacheCreationTTLTokensFromUsage(t *testing.T) {
	for _, tc := range []struct {
		name, details string
		short, long   int
	}{
		{"mixed", `{"ttl_30m_tokens":2,"ttl_12h_tokens":8}`, 2, 8},
		{"short only", `{"ttl_30m_tokens":10,"ttl_12h_tokens":0}`, 10, 0},
		{"long only", `{"ttl_30m_tokens":0,"ttl_12h_tokens":10}`, 0, 10},
		{"missing", `{}`, 0, 0},
		{"partial", `{"ttl_12h_tokens":10}`, 0, 0},
		{"inconsistent", `{"ttl_30m_tokens":2,"ttl_12h_tokens":9}`, 0, 0},
		{"negative", `{"ttl_30m_tokens":-1,"ttl_12h_tokens":11}`, 0, 0},
		{"fractional", `{"ttl_30m_tokens":0.5,"ttl_12h_tokens":9.5}`, 0, 0},
		{"string", `{"ttl_30m_tokens":"0","ttl_12h_tokens":"10"}`, 0, 0},
	} {
		for _, path := range []string{"input_tokens_details", "prompt_tokens_details"} {
			t.Run(tc.name+"/"+path, func(t *testing.T) {
				value := gjson.Parse(`{"` + path + `":{"cache_write_tokens_details":` + tc.details + `}}`)
				short, long := CacheCreationTTLTokensFromUsage(value, 10)
				require.Equal(t, tc.short, short)
				require.Equal(t, tc.long, long)
			})
		}
	}
}
