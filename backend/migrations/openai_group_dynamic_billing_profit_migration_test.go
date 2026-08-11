package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGroupDynamicBillingProfitMigration(t *testing.T) {
	sql, err := FS.ReadFile("222_openai_group_dynamic_billing_profit.sql")
	require.NoError(t, err)

	text := string(sql)
	require.Contains(t, text, "dynamic_billing_profit_markup DECIMAL(12,6)")
	require.Contains(t, text, "OLD.dynamic_billing_profit_markup IS NOT DISTINCT FROM NEW.dynamic_billing_profit_markup")
}
