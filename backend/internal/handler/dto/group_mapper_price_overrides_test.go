package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupFromServicePreservesPriceOverrides(t *testing.T) {
	for _, tc := range []struct {
		name  string
		price *float64
	}{
		{name: "configured", price: priceOverridePointer(1.25)},
		{name: "explicit zero", price: priceOverridePointer(0)},
		{name: "default", price: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &service.Group{
				SearchPricePer1k:             tc.price,
				AudioRealtimePricePerMin:     tc.price,
				AudioTTSPricePerMillionChars: tc.price,
				AudioSTTPricePerHour:         tc.price,
			}
			if tc.price != nil {
				group.VideoModelPrices = map[string]map[string]float64{
					"veo-3": {"720p": *tc.price, "1080p": 2.5},
				}
			}
			for name, mapped := range map[string]any{
				"admin":   GroupFromServiceAdmin(group),
				"user":    GroupFromService(group),
				"shallow": GroupFromServiceShallow(group),
			} {
				t.Run(name, func(t *testing.T) {
					fields := marshalToMap(t, mapped)
					for _, key := range []string{
						"search_price_per_1k",
						"audio_realtime_price_per_min",
						"audio_tts_price_per_million_chars",
						"audio_stt_price_per_hour",
					} {
						require.Contains(t, fields, key)
						if tc.price == nil {
							require.Nil(t, fields[key], key)
						} else {
							require.Equal(t, *tc.price, fields[key], key)
						}
					}
					if tc.price == nil {
						require.NotContains(t, fields, "video_model_prices")
					} else {
						require.Equal(t, map[string]any{
							"veo-3": map[string]any{"720p": *tc.price, "1080p": 2.5},
						}, fields["video_model_prices"])
					}
				})
			}
		})
	}
}

func priceOverridePointer(price float64) *float64 {
	return &price
}
