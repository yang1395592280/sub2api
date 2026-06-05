package service

import "testing"

func TestSizeBetDirectionForNumber(t *testing.T) {
	if got, ok := SizeBetDirectionForNumber(6); !ok || got != SizeBetDirectionMid {
		t.Fatalf("expected mid for 6, got %v %v", got, ok)
	}
	if _, ok := SizeBetDirectionForNumber(0); ok {
		t.Fatalf("expected invalid number")
	}
}
