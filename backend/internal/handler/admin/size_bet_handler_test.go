package admin

import (
	"testing"
	"time"
)

func TestFormatAdminOptionalTimeNil(t *testing.T) {
	if got := formatAdminOptionalTime(nil); got != nil {
		t.Fatalf("expected nil")
	}
}

func TestFormatAdminOptionalTimeValue(t *testing.T) {
	now := time.Date(2026, 4, 24, 1, 2, 3, 0, time.UTC)
	if got := formatAdminOptionalTime(&now); got == nil || *got == "" {
		t.Fatalf("expected formatted time")
	}
}
