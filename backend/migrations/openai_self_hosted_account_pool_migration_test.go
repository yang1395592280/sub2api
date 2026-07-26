package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAISelfHostedAccountPoolMigrationContract(t *testing.T) {
	migration, err := FS.ReadFile("191_openai_self_hosted_account_pool.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(migration))
	require.Contains(t, sql, "group_role varchar(32) not null default 'standard'")
	require.Contains(t, sql, "self_hosted_pool_group_id bigint")
	require.Contains(t, sql, "check (group_role in ('standard', 'self_hosted_pool'))")
	require.Contains(t, sql, "foreign key (self_hosted_pool_group_id)")
	require.Contains(t, sql, "references groups(id)")
	require.Contains(t, sql, "on delete restrict")
	require.Contains(t, sql, "create index if not exists idx_groups_self_hosted_pool_group_id")
	require.Contains(t, sql, "effective_group_id bigint")
	require.Contains(t, sql, "account_source_group_id bigint")
	require.Contains(t, sql, "account_source_type varchar(32)")
	require.Contains(t, sql, "pool_group_id bigint")
	require.Contains(t, sql, "pool_fallback_reason varchar(64)")
	require.NotContains(t, sql, "unique index")
	require.NotContains(t, sql, "unique (self_hosted_pool_group_id)")
}
