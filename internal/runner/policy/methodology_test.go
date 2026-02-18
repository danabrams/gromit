package policy_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/policy"
)

func newMethodologyPolicy(atdd, tdd bool) policy.MethodologyPolicy {
	cfg := &config.Config{}
	cfg.Methodology.ATDD = atdd
	cfg.Methodology.TDD = tdd
	return policy.NewConfigMethodologyPolicy(cfg)
}

func TestIsActive_TrueLabelOverridesGlobalFalse(t *testing.T) {
	p := newMethodologyPolicy(false, false)
	labels := []string{"tdd:true"}
	if !p.IsActive(labels, "tdd") {
		t.Error("expected IsActive to return true when tdd:true label present")
	}
}
