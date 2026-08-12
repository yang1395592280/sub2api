package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateGroupRequestDynamicBillingProfitMarkupTriState(t *testing.T) {
	t.Run("omitted keeps existing override", func(t *testing.T) {
		var req UpdateGroupRequest
		require.NoError(t, json.Unmarshal([]byte(`{}`), &req))
		require.False(t, req.DynamicBillingProfitMarkup.set)
	})

	t.Run("null clears override and inherits global", func(t *testing.T) {
		var req UpdateGroupRequest
		require.NoError(t, json.Unmarshal([]byte(`{"dynamic_billing_profit_markup":null}`), &req))
		require.True(t, req.DynamicBillingProfitMarkup.set)
		require.Nil(t, req.DynamicBillingProfitMarkup.value)
	})

	t.Run("zero explicitly disables group profit", func(t *testing.T) {
		var req UpdateGroupRequest
		require.NoError(t, json.Unmarshal([]byte(`{"dynamic_billing_profit_markup":0}`), &req))
		require.True(t, req.DynamicBillingProfitMarkup.set)
		require.NotNil(t, req.DynamicBillingProfitMarkup.value)
		require.Zero(t, *req.DynamicBillingProfitMarkup.value)
	})
}
