package worktree

import (
	"errors"
	"testing"
)

func TestClassifySessionCreateFailure(t *testing.T) {
	tests := []struct {
		name   string
		output string
		err    error
		want   sessionCreateFailureClass
	}{
		{
			name: "branch exists retryable",
			err:  errors.New("fatal: a branch named 'gromit/review-100' already exists"),
			want: sessionCreateFailureRetryable,
		},
		{
			name: "worktree already used retryable",
			err:  errors.New("fatal: 'gromit/review-100' is already used by worktree at '/tmp/repo'"),
			want: sessionCreateFailureRetryable,
		},
		{
			name: "ambiguous lock-ref requires probes",
			err:  errors.New("fatal: cannot lock ref 'refs/heads/gromit/review-100': 'refs/heads/gromit/review-100' exists; cannot create 'refs/heads/gromit/review-100'"),
			want: sessionCreateFailureAmbiguousProbe,
		},
		{
			name: "terminal non contention",
			err:  errors.New("fatal: remote 'origin' already exists"),
			want: sessionCreateFailureTerminal,
		},
		{
			name:   "uses output before wrapped error text",
			output: "fatal: a branch named 'gromit/review-100' already exists",
			err:    errors.New("exit status 128"),
			want:   sessionCreateFailureRetryable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySessionCreateFailure(tt.output, tt.err)
			if got != tt.want {
				t.Fatalf("classifySessionCreateFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}
