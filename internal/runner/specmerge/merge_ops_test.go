package specmerge_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/runner/specmerge"
)

func TestFinalizeSpecBranch_RebaseBeforeMergeBeforeDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	branch := "gromit/spec-payments"
	callLog := []string{}

	git := &fakeGitOps{
		rebaseFn: func(_ context.Context, b, onto string) error {
			callLog = append(callLog, fmt.Sprintf("rebase %s %s", b, onto))
			return nil
		},
		mergeFn: func(_ context.Context, b string) error {
			callLog = append(callLog, fmt.Sprintf("merge %s", b))
			return nil
		},
		deleteFn: func(_ context.Context, b string) error {
			callLog = append(callLog, fmt.Sprintf("delete %s", b))
			return nil
		},
	}

	deps := specmerge.FinalizeDependencies{
		Git: git,
	}

	if err := specmerge.FinalizeSpecBranch(ctx, deps, branch); err != nil {
		t.Fatalf("FinalizeSpecBranch returned %v", err)
	}

	want := []string{
		"rebase gromit/spec-payments main",
		"merge gromit/spec-payments",
		"delete gromit/spec-payments",
	}
	if !reflect.DeepEqual(callLog, want) {
		t.Fatalf("call order = %v, want %v", callLog, want)
	}
}

type fakeGitOps struct {
	rebaseFn func(ctx context.Context, branch, onto string) error
	mergeFn  func(ctx context.Context, branch string) error
	deleteFn func(ctx context.Context, branch string) error
}

func (f *fakeGitOps) RebaseOnto(ctx context.Context, branch, onto string) error {
	if f.rebaseFn == nil {
		return nil
	}
	return f.rebaseFn(ctx, branch, onto)
}

func (f *fakeGitOps) FastForwardMerge(ctx context.Context, branch string) error {
	if f.mergeFn == nil {
		return nil
	}
	return f.mergeFn(ctx, branch)
}

func (f *fakeGitOps) DeleteBranch(ctx context.Context, branch string) error {
	if f.deleteFn == nil {
		return nil
	}
	return f.deleteFn(ctx, branch)
}
