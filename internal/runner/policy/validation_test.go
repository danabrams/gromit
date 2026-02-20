package policy_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/policy"
)

func newConfigValidationPolicy(fullEveryN int) policy.ValidationPolicy {
	cfg := &config.Config{}
	cfg.Validation.FullValidationEveryN = fullEveryN
	return policy.NewConfigValidationPolicy(cfg)
}

func newConfigValidationPolicyWithMandatory(fullEveryN int, mandatory []string) policy.ValidationPolicy {
	cfg := &config.Config{}
	cfg.Validation.FullValidationEveryN = fullEveryN
	cfg.Validation.MandatoryCommands = mandatory
	return policy.NewConfigValidationPolicy(cfg)
}

func TestSelectGate_ReturnsFastOnNonModuloIteration(t *testing.T) {
	p := newConfigValidationPolicy(3)
	if got := p.SelectGate(1); got != policy.GateFast {
		t.Errorf("SelectGate(1) with fullEveryN=3: got %v, want GateFast", got)
	}
}

func TestSelectGate_ReturnsFullOnModuloBoundary(t *testing.T) {
	p := newConfigValidationPolicy(3)
	if got := p.SelectGate(3); got != policy.GateFull {
		t.Errorf("SelectGate(3) with fullEveryN=3: got %v, want GateFull", got)
	}
}

func TestSelectGate_ZeroConsecutiveSuccessesReturnsFullWhenEnabled(t *testing.T) {
	p := newConfigValidationPolicy(3)
	if got := p.SelectGate(0); got != policy.GateFull {
		t.Errorf("SelectGate(0) with fullEveryN=3: got %v, want GateFull (0%%3==0)", got)
	}
}

func TestSelectGate_ZeroFullEveryNAlwaysReturnsFast(t *testing.T) {
	p := newConfigValidationPolicy(0)
	for _, n := range []int{0, 1, 5, 100} {
		if got := p.SelectGate(n); got != policy.GateFast {
			t.Errorf("SelectGate(%d) with fullEveryN=0: got %v, want GateFast", n, got)
		}
	}
}

func TestMaxRecoveryAttempts_ReturnsTwo(t *testing.T) {
	p := newConfigValidationPolicy(0)
	if got := p.MaxRecoveryAttempts(); got != 2 {
		t.Errorf("MaxRecoveryAttempts: got %d, want 2", got)
	}
}

func TestShouldEscalateRecovery_ReturnsTrue(t *testing.T) {
	p := newConfigValidationPolicy(0)
	if !p.ShouldEscalateRecovery() {
		t.Error("ShouldEscalateRecovery: expected true")
	}
}

func TestSelectGate_TableDrivenModuloArithmetic(t *testing.T) {
	tests := []struct {
		name                 string
		fullEveryN           int
		consecutiveSuccesses int
		want                 policy.GateType
	}{
		{"fullEveryN=1 always full at 0", 1, 0, policy.GateFull},
		{"fullEveryN=1 always full at 1", 1, 1, policy.GateFull},
		{"fullEveryN=5 fast at 1", 5, 1, policy.GateFast},
		{"fullEveryN=5 fast at 4", 5, 4, policy.GateFast},
		{"fullEveryN=5 full at 5", 5, 5, policy.GateFull},
		{"fullEveryN=5 fast at 6", 5, 6, policy.GateFast},
		{"fullEveryN=5 full at 10", 5, 10, policy.GateFull},
		{"fullEveryN=0 disabled fast at 0", 0, 0, policy.GateFast},
		{"fullEveryN=0 disabled fast at 5", 0, 5, policy.GateFast},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newConfigValidationPolicy(tc.fullEveryN)
			got := p.SelectGate(tc.consecutiveSuccesses)
			if got != tc.want {
				t.Errorf("SelectGate(%d) with fullEveryN=%d: got %v, want %v",
					tc.consecutiveSuccesses, tc.fullEveryN, got, tc.want)
			}
		})
	}
}

func TestMandatoryCommandPrefixes_ReturnsConfiguredPrefixes(t *testing.T) {
	p := newConfigValidationPolicyWithMandatory(0, []string{"go test", "go vet", "go build"})
	want := []string{"go test", "go vet", "go build"}
	got := p.MandatoryCommandPrefixes()
	if len(got) != len(want) {
		t.Fatalf("MandatoryCommandPrefixes: got %v, want %v", got, want)
	}
	for i, prefix := range want {
		if got[i] != prefix {
			t.Errorf("MandatoryCommandPrefixes[%d]: got %q, want %q", i, got[i], prefix)
		}
	}
}

func TestMandatoryCommandPrefixes_EmptyWhenUnconfigured(t *testing.T) {
	p := newConfigValidationPolicy(0)
	got := p.MandatoryCommandPrefixes()
	if len(got) != 0 {
		t.Fatalf("MandatoryCommandPrefixes: got %v, want empty", got)
	}
}
