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

func TestClassifyTimeout_ParentCanceledReturnsNoTimeoutType(t *testing.T) {
	p := newConfigEscalationPolicy(&config.Config{})
	parentErr := context.Canceled
	result := p.ClassifyTimeout(nil, parentErr, false)
	if result.TimeoutType != "" {
		t.Errorf("ClassifyTimeout(parent canceled) TimeoutType = %q, want empty", result.TimeoutType)
	}
	if !result.ParentCanceled {
		t.Error("ClassifyTimeout(parent canceled) ParentCanceled = false, want true")
	}
}

func TestClassifyTimeout_DefaultsToInvocation(t *testing.T) {
	p := newConfigEscalationPolicy(&config.Config{})
	result := p.ClassifyTimeout(nil, nil, false)
	if result.TimeoutType != "invocation" {
		t.Errorf("ClassifyTimeout(default) TimeoutType = %q, want %q", result.TimeoutType, "invocation")
	}
	if result.ParentCanceled {
		t.Error("ClassifyTimeout(default) ParentCanceled = true, want false")
	}
}

func TestClassifyTimeout_AllBranchCombinations(t *testing.T) {
	errCtx := context.DeadlineExceeded
	errParent := context.Canceled
	p := newConfigEscalationPolicy(&config.Config{})
	cases := []struct {
		name           string
		ctxErr         error
		parentErr      error
		stallFired     bool
		wantType       string
		wantParentStop bool
	}{
		{"no errors no stall", nil, nil, false, "invocation", false},
		{"stall only", nil, nil, true, "stall", false},
		{"ctx expired", errCtx, nil, false, "bead", false},
		{"ctx expired with stall", errCtx, nil, true, "bead", false},
		{"parent canceled", nil, errParent, false, "", true},
		{"parent canceled with stall", nil, errParent, true, "stall", false},
		{"ctx + parent canceled", errCtx, errParent, false, "", true},
		{"ctx + parent canceled with stall", errCtx, errParent, true, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := p.ClassifyTimeout(tc.ctxErr, tc.parentErr, tc.stallFired)
			if result.TimeoutType != tc.wantType {
				t.Errorf("TimeoutType = %q, want %q", result.TimeoutType, tc.wantType)
			}
			if result.ParentCanceled != tc.wantParentStop {
				t.Errorf("ParentCanceled = %v, want %v", result.ParentCanceled, tc.wantParentStop)
			}
		})
	}
}

func TestSelectInitialTier_DelegatesToConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Models.P1 = "low"
	p := newConfigEscalationPolicy(cfg)
	want := cfg.SelectTier(1, nil)
	got := p.SelectInitialTier(1, nil)
	if got != want {
		t.Errorf("SelectInitialTier() = %q, want %q", got, want)
	}
}

func TestSelectModel_DelegatesToConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Models.P2 = "haiku"
	p := newConfigEscalationPolicy(cfg)
	want := cfg.SelectModel(2, nil)
	got := p.SelectModel(2, nil)
	if got != want {
		t.Errorf("SelectModel() = %q, want %q", got, want)
	}
}

func TestNextTier_DelegatesToConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Escalation.Enabled = true
	cfg.Escalation.Chain = []string{"low", "medium", "high"}
	p := newConfigEscalationPolicy(cfg)
	want := cfg.NextEscalationTier("low")
	got := p.NextTier("low")
	if got != want {
		t.Errorf("NextTier() = %q, want %q", got, want)
	}
}

func TestMaxRetriesPerModel_DelegatesToConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Escalation.MaxRetriesPerModel = 4
	p := newConfigEscalationPolicy(cfg)
	if got := p.MaxRetriesPerModel(); got != 4 {
		t.Errorf("MaxRetriesPerModel() = %d, want %d", got, 4)
	}
}

func TestMaxRetriesPerBead_DelegatesToConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Escalation.MaxRetriesPerBead = 9
	p := newConfigEscalationPolicy(cfg)
	if got := p.MaxRetriesPerBead(); got != 9 {
		t.Errorf("MaxRetriesPerBead() = %d, want %d", got, 9)
	}
}
