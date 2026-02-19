package runner

import "github.com/danabrams/gromit/internal/runner/policy"

func (r *Runner) ensureValidationPolicy() policy.ValidationPolicy {
	if r == nil {
		return nil
	}
	if r.validationPolicy == nil && r.cfg != nil {
		r.validationPolicy = policy.NewConfigValidationPolicy(r.cfg)
	}
	return r.validationPolicy
}
