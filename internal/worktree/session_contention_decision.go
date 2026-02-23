package worktree

type sessionCreateRetryInput struct {
	FailureClass            sessionCreateFailureClass
	ProbeBranchExists       func() bool
	ProbeWorktreeRegistered func() bool
}

type sessionCreateRetryDecision struct {
	Retry            bool
	BranchProbeRan   bool
	BranchExists     bool
	WorktreeProbeRan bool
	WorktreeExists   bool
}

func decideSessionCreateRetry(input sessionCreateRetryInput) sessionCreateRetryDecision {
	switch input.FailureClass {
	case sessionCreateFailureRetryable:
		return sessionCreateRetryDecision{Retry: true}
	case sessionCreateFailureAmbiguousProbe:
		decision := sessionCreateRetryDecision{}
		if input.ProbeBranchExists != nil {
			decision.BranchProbeRan = true
			decision.BranchExists = input.ProbeBranchExists()
		}
		if decision.BranchExists {
			decision.Retry = true
			return decision
		}
		if input.ProbeWorktreeRegistered != nil {
			decision.WorktreeProbeRan = true
			decision.WorktreeExists = input.ProbeWorktreeRegistered()
		}
		decision.Retry = decision.WorktreeExists
		return decision
	default:
		return sessionCreateRetryDecision{}
	}
}
