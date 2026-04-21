package repository

import (
	"strings"
	"testing"

	dbentsql "entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

func TestAccountListOrderUsesRateLimitResetAtForRecoverSort(t *testing.T) {
	t.Parallel()

	selector := dbentsql.Select("*").From(dbentsql.Table("accounts"))
	for _, order := range accountListOrder(pagination.PaginationParams{
		SortBy:    "recover_at",
		SortOrder: "asc",
	}, "rate_limited") {
		order(selector)
	}
	query, _ := selector.Query()

	if !strings.Contains(query, "rate_limit_reset_at") {
		t.Fatalf("expected recover_at sort to use rate_limit_reset_at, got query: %s", query)
	}
}

func TestAccountListOrderUsesTempUnschedulableUntilForRecoverSort(t *testing.T) {
	t.Parallel()

	selector := dbentsql.Select("*").From(dbentsql.Table("accounts"))
	for _, order := range accountListOrder(pagination.PaginationParams{
		SortBy:    "recover_at",
		SortOrder: "desc",
	}, "temp_unschedulable") {
		order(selector)
	}
	query, _ := selector.Query()

	if !strings.Contains(query, "temp_unschedulable_until") {
		t.Fatalf("expected recover_at sort to use temp_unschedulable_until, got query: %s", query)
	}
}
