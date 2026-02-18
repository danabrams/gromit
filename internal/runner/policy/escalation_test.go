package policy_test

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/policy"
)

func newConfigEscalationPolicy(cfg *config.Config) policy.EscalationPolicy {
	return policy.NewConfigEscalationPolicy(cfg)
}

func TestClassifyTimeout_StallFiredWithoutContextError(t *testing.T) {
	p := newConfigEscalationPolicy(&config.Config{})
	result := p.ClassifyTimeout(nil, nil, true)
	if result.TimeoutType != "stall" {
		t.Errorf("ClassifyTimeout(stall only) TimeoutType = %q, want %q", result.TimeoutType, "stall")
	}
	if result.ParentCanceled {
		t.Error("ClassifyTimeout(stall only) ParentCanceled = true, want false")
	}
}

func TestClassifyTimeout_BeadTimeoutWhenContextExpired(t *testing.T) {
	p := newConfigEscalationPolicy(&config.Config{})
	ctxErr := context.DeadlineExceeded
	result := p.ClassifyTimeout(ctxErr, nil, false)
	if result.TimeoutType != "bead" {
		t.Errorf("ClassifyTimeout(bead) TimeoutType = %q, want %q", result.TimeoutType, "bead")
	}
	if result.ParentCanceled {
		t.Error("ClassifyTimeout(bead) ParentCanceled = true, want false")
	}
}
