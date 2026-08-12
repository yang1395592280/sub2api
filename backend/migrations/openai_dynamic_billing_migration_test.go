package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIDynamicBillingMigrationAddsOptInGroupFlag(t *testing.T) {
	sql, err := FS.ReadFile("221_openai_dynamic_billing.sql")
	require.NoError(t, err)
	require.Contains(t, string(sql), "dynamic_billing_enabled BOOLEAN NOT NULL DEFAULT FALSE")
}
