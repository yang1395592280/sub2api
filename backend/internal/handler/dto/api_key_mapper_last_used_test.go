package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyFromService_MapsLastUsedAt(t *testing.T) {
	lastUsed := time.Now().UTC().Truncate(time.Second)
	src := &service.APIKey{
		ID:         1,
		UserID:     2,
		Key:        "sk-map-last-used",
		Name:       "Mapper",
		Status:     service.StatusActive,
		LastUsedAt: &lastUsed,
	}

	out := APIKeyFromService(src)
	require.NotNil(t, out)
	require.NotNil(t, out.LastUsedAt)
	require.WithinDuration(t, lastUsed, *out.LastUsedAt, time.Second)
}

func TestAPIKeyFromService_MapsNilLastUsedAt(t *testing.T) {
	src := &service.APIKey{
		ID:     1,
		UserID: 2,
		Key:    "sk-map-last-used-nil",
		Name:   "MapperNil",
		Status: service.StatusActive,
	}

	out := APIKeyFromService(src)
	require.NotNil(t, out)
	require.Nil(t, out.LastUsedAt)
}

func TestAPIKeyFromService_MapsGroupSelectionFields(t *testing.T) {
	lastEffectiveGroupID := int64(9)
	lastEffectiveGroupAt := time.Now().UTC().Truncate(time.Second)
	maxRate := 0.8
	src := &service.APIKey{
		ID:                               1,
		UserID:                           2,
		Key:                              "sk-map-group-mode",
		Name:                             "MapperMode",
		Status:                           service.StatusActive,
		GroupSelectMode:                  service.APIKeyGroupSelectModeOpenAIAutoCheapest,
		OpenAIAutoGroupMaxRateMultiplier: &maxRate,
		LastEffectiveGroupID:             &lastEffectiveGroupID,
		LastEffectiveGroupAt:             &lastEffectiveGroupAt,
		LastEffectiveGroup: &service.Group{
			ID:       lastEffectiveGroupID,
			Name:     "openai-auto",
			Platform: service.PlatformOpenAI,
			Status:   service.StatusActive,
		},
	}

	out := APIKeyFromService(src)
	require.NotNil(t, out)
	require.Equal(t, service.APIKeyGroupSelectModeOpenAIAutoCheapest, out.GroupSelectMode)
	require.Equal(t, &maxRate, out.OpenAIAutoGroupMaxRateMultiplier)
	require.Equal(t, &lastEffectiveGroupID, out.LastEffectiveGroupID)
	require.NotNil(t, out.LastEffectiveGroupAt)
	require.WithinDuration(t, lastEffectiveGroupAt, *out.LastEffectiveGroupAt, time.Second)
	require.NotNil(t, out.LastEffectiveGroup)
	require.Equal(t, lastEffectiveGroupID, out.LastEffectiveGroup.ID)
}
