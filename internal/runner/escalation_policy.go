package runner

import "github.com/danabrams/gromit/internal/runner/policy"

func (r *Runner) ensureEscalationPolicy() policy.EscalationPolicy {
	if r == nil {
		return nil
	}
	if r.escalationPolicy == nil && r.cfg != nil {
		r.escalationPolicy = policy.NewConfigEscalationPolicy(r.cfg)
	}
	return r.escalationPolicy
}
