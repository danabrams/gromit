package gate

import (
	"testing"
)

func TestSatisfactionTier_Gen0SkipsCheck(t *testing.T) {
	if got := satisfactionTier(0); got != "" {
		t.Errorf("satisfactionTier(0) = %q, want %q", got, "")
	}
}

func TestSatisfactionTier_Gen1ReturnsLow(t *testing.T) {
	if got := satisfactionTier(1); got != "low" {
		t.Errorf("satisfactionTier(1) = %q, want %q", got, "low")
	}
}

func TestSatisfactionTier_Gen2ReturnsMedium(t *testing.T) {
	if got := satisfactionTier(2); got != "medium" {
		t.Errorf("satisfactionTier(2) = %q, want %q", got, "medium")
	}
}

func TestSatisfactionTier_Gen3ReturnsHigh(t *testing.T) {
	if got := satisfactionTier(3); got != "high" {
		t.Errorf("satisfactionTier(3) = %q, want %q", got, "high")
	}
}

func TestSatisfactionTier_Gen5ReturnsHigh(t *testing.T) {
	if got := satisfactionTier(5); got != "high" {
		t.Errorf("satisfactionTier(5) = %q, want %q", got, "high")
	}
}
