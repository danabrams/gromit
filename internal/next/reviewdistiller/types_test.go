package reviewdistiller

import (
	"testing"
)

// TestTierLowConstant verifies TierLow has the correct value.
func TestTierLowConstant(t *testing.T) {
	expected := Tier("low")
	if TierLow != expected {
		t.Errorf("TierLow = %q, want %q", TierLow, expected)
	}
}

// TestTierMediumConstant verifies TierMedium has the correct value.
func TestTierMediumConstant(t *testing.T) {
	expected := Tier("medium")
	if TierMedium != expected {
		t.Errorf("TierMedium = %q, want %q", TierMedium, expected)
	}
}

// TestTierHighConstant verifies TierHigh has the correct value.
func TestTierHighConstant(t *testing.T) {
	expected := Tier("high")
	if TierHigh != expected {
		t.Errorf("TierHigh = %q, want %q", TierHigh, expected)
	}
}
