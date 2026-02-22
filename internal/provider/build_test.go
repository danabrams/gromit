package provider

import "testing"

func TestDefaultTierToModelMap(t *testing.T) {
	if got := DefaultTierToModelMap[TierHigh]; got != "opus" {
		t.Fatalf("DefaultTierToModelMap[%q] = %q, want %q", TierHigh, got, "opus")
	}
	if got := DefaultTierToModelMap[TierMedium]; got != "sonnet" {
		t.Fatalf("DefaultTierToModelMap[%q] = %q, want %q", TierMedium, got, "sonnet")
	}
	if got := DefaultTierToModelMap[TierLow]; got != "haiku" {
		t.Fatalf("DefaultTierToModelMap[%q] = %q, want %q", TierLow, got, "haiku")
	}
}
