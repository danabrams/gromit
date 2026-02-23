package worktree

import "testing"

func TestDecideSessionCreateRetry_AmbiguousBranchProbePositive(t *testing.T) {
	decision := decideSessionCreateRetry(sessionCreateRetryInput{
		FailureClass: sessionCreateFailureAmbiguousProbe,
		ProbeBranchExists: func() bool {
			return true
		},
		ProbeWorktreeRegistered: func() bool {
			t.Fatal("worktree probe should not run when branch probe is positive")
			return false
		},
	})

	if !decision.Retry {
		t.Fatal("decideSessionCreateRetry() retry = false, want true")
	}
	if !decision.BranchProbeRan || !decision.BranchExists {
		t.Fatal("expected positive branch probe to be recorded")
	}
	if decision.WorktreeProbeRan {
		t.Fatal("expected worktree probe to be skipped when branch probe is positive")
	}
}
