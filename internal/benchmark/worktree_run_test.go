package benchmark

import (
	"context"
	"testing"
)

func TestGitBaseCommitResolver_ResolvesHintWithRevParse(t *testing.T) {
	t.Parallel()

	calls := make([][]string, 0, 1)
	resolver := NewGitBaseCommitResolver(func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "abc123\n", nil
	})

	got, err := resolver.ResolveBaseCommit(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("ResolveBaseCommit() error = %v", err)
	}
	if got != "abc123" {
		t.Fatalf("ResolveBaseCommit() = %q, want %q", got, "abc123")
	}
	if len(calls) != 1 {
		t.Fatalf("git call count = %d, want 1", len(calls))
	}
	if len(calls[0]) != 3 || calls[0][0] != "rev-parse" || calls[0][1] != "--verify" || calls[0][2] != "feature-branch" {
		t.Fatalf("git args = %v, want [rev-parse --verify feature-branch]", calls[0])
	}
}
