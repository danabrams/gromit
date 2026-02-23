package worktree

import "testing"

func TestDecideSessionCreateRetry(t *testing.T) {
	tests := []struct {
		name      string
		input     sessionCreateRetryInput
		assertion func(t *testing.T, got sessionCreateRetryDecision)
	}{
		{
			name: "retryable class retries immediately",
			input: sessionCreateRetryInput{
				FailureClass: sessionCreateFailureRetryable,
			},
			assertion: func(t *testing.T, got sessionCreateRetryDecision) {
				if !got.Retry {
					t.Fatal("decideSessionCreateRetry() retry = false, want true")
				}
				if got.BranchProbeRan || got.WorktreeProbeRan {
					t.Fatal("retryable class should not run probes")
				}
			},
		},
		{
			name: "ambiguous branch probe positive short circuits",
			input: sessionCreateRetryInput{
				FailureClass: sessionCreateFailureAmbiguousProbe,
				ProbeBranchExists: func() bool {
					return true
				},
				ProbeWorktreeRegistered: func() bool {
					t.Fatal("worktree probe should not run when branch probe is positive")
					return false
				},
			},
			assertion: func(t *testing.T, got sessionCreateRetryDecision) {
				if !got.Retry {
					t.Fatal("decideSessionCreateRetry() retry = false, want true")
				}
				if !got.BranchProbeRan || !got.BranchExists {
					t.Fatal("expected positive branch probe to be recorded")
				}
				if got.WorktreeProbeRan {
					t.Fatal("expected worktree probe to be skipped when branch probe is positive")
				}
			},
		},
		{
			name: "ambiguous worktree probe positive retries",
			input: sessionCreateRetryInput{
				FailureClass: sessionCreateFailureAmbiguousProbe,
				ProbeBranchExists: func() bool {
					return false
				},
				ProbeWorktreeRegistered: func() bool {
					return true
				},
			},
			assertion: func(t *testing.T, got sessionCreateRetryDecision) {
				if !got.Retry {
					t.Fatal("decideSessionCreateRetry() retry = false, want true")
				}
				if !got.BranchProbeRan || got.BranchExists {
					t.Fatal("expected branch probe to run and return false")
				}
				if !got.WorktreeProbeRan || !got.WorktreeExists {
					t.Fatal("expected worktree probe to run and return true")
				}
				if got.TerminalReason != "" {
					t.Fatalf("terminal reason = %q, want empty", got.TerminalReason)
				}
			},
		},
		{
			name: "ambiguous probes inconclusive",
			input: sessionCreateRetryInput{
				FailureClass: sessionCreateFailureAmbiguousProbe,
				ProbeBranchExists: func() bool {
					return false
				},
				ProbeWorktreeRegistered: func() bool {
					return false
				},
			},
			assertion: func(t *testing.T, got sessionCreateRetryDecision) {
				if got.Retry {
					t.Fatal("decideSessionCreateRetry() retry = true, want false for inconclusive probes")
				}
				if got.TerminalReason != "ambiguous_probe_inconclusive" {
					t.Fatalf("decideSessionCreateRetry() terminal reason = %q, want %q", got.TerminalReason, "ambiguous_probe_inconclusive")
				}
			},
		},
		{
			name: "terminal class returns non retryable reason",
			input: sessionCreateRetryInput{
				FailureClass: sessionCreateFailureTerminal,
			},
			assertion: func(t *testing.T, got sessionCreateRetryDecision) {
				if got.Retry {
					t.Fatal("decideSessionCreateRetry() retry = true, want false")
				}
				if got.TerminalReason != "non_retryable_failure" {
					t.Fatalf("decideSessionCreateRetry() terminal reason = %q, want %q", got.TerminalReason, "non_retryable_failure")
				}
			},
		},
		{
			name: "ambiguous with nil probes is inconclusive",
			input: sessionCreateRetryInput{
				FailureClass: sessionCreateFailureAmbiguousProbe,
			},
			assertion: func(t *testing.T, got sessionCreateRetryDecision) {
				if got.Retry {
					t.Fatal("decideSessionCreateRetry() retry = true, want false")
				}
				if got.BranchProbeRan || got.WorktreeProbeRan {
					t.Fatal("nil probes should not be marked as run")
				}
				if got.TerminalReason != "ambiguous_probe_inconclusive" {
					t.Fatalf("decideSessionCreateRetry() terminal reason = %q, want %q", got.TerminalReason, "ambiguous_probe_inconclusive")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideSessionCreateRetry(tt.input)
			tt.assertion(t, got)
		})
	}
}
