package specmerge_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/specmerge"
)

func withDefaultBaseBranch(t *testing.T, branch string) func() {
	t.Helper()
	original := config.DefaultBaseBranch
	config.DefaultBaseBranch = branch
	return func() {
		config.DefaultBaseBranch = original
	}
}

func TestFinalizeSpecBranch_RebaseBeforeMergeBeforeDelete(t *testing.T) {
	ctx := context.Background()
	branch := "gromit/spec-payments"
	callLog := []string{}
	defer withDefaultBaseBranch(t, "test-main-branch")()

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
		fmt.Sprintf("rebase %s %s", branch, config.DefaultBaseBranch),
		"merge gromit/spec-payments",
		"delete gromit/spec-payments",
	}
	if !reflect.DeepEqual(callLog, want) {
		t.Fatalf("call order = %v, want %v", callLog, want)
	}
}

func TestFinalizeSpecBranch_RebaseConflictTriggersResolver(t *testing.T) {
	ctx := context.Background()
	branch := "gromit/spec-payments"
	defer withDefaultBaseBranch(t, "test-main-branch")()
	callLog := []string{}
	rebaseAttempts := 0

	git := &fakeGitOps{
		rebaseFn: func(_ context.Context, b, onto string) error {
			rebaseAttempts++
			callLog = append(callLog, fmt.Sprintf("rebase %d %s", rebaseAttempts, onto))
			if rebaseAttempts == 1 {
				return &specmerge.ConflictError{
					Operation: "rebase",
					Err:       fmt.Errorf("merge conflict"),
				}
			}
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

	resolverCalled := 0
	resolver := &fakeResolver{
		resolveFn: func(_ context.Context, b string, cause error) error {
			resolverCalled++
			if b != branch {
				t.Fatalf("resolver branch = %q, want %q", b, branch)
			}
			var conflictErr *specmerge.ConflictError
			if !errors.As(cause, &conflictErr) {
				t.Fatalf("resolve cause = %v, want ConflictError", cause)
			}
			callLog = append(callLog, "resolve")
			return nil
		},
	}

	deps := specmerge.FinalizeDependencies{
		Git:              git,
		ConflictResolver: resolver,
	}

	if err := specmerge.FinalizeSpecBranch(ctx, deps, branch); err != nil {
		t.Fatalf("FinalizeSpecBranch returned %v", err)
	}
	if resolverCalled != 1 {
		t.Fatalf("resolver called %d times, want 1", resolverCalled)
	}

	want := []string{
		fmt.Sprintf("rebase 1 %s", config.DefaultBaseBranch),
		"resolve",
		fmt.Sprintf("rebase 2 %s", config.DefaultBaseBranch),
		"merge gromit/spec-payments",
		"delete gromit/spec-payments",
	}
	if !reflect.DeepEqual(callLog, want) {
		t.Fatalf("call order = %v, want %v", callLog, want)
	}
}

func TestFinalizeSpecBranch_MergeConflictTriggersResolver(t *testing.T) {
	ctx := context.Background()
	branch := "gromit/spec-payments"
	defer withDefaultBaseBranch(t, "test-main-branch")()
	callLog := []string{}
	mergeAttempts := 0

	git := &fakeGitOps{
		rebaseFn: func(_ context.Context, b, onto string) error {
			callLog = append(callLog, fmt.Sprintf("rebase %s %s", b, onto))
			return nil
		},
		mergeFn: func(_ context.Context, b string) error {
			mergeAttempts++
			callLog = append(callLog, fmt.Sprintf("merge %d %s", mergeAttempts, b))
			if mergeAttempts == 1 {
				return &specmerge.ConflictError{
					Operation: "merge",
					Err:       fmt.Errorf("fast-forward blocked"),
				}
			}
			return nil
		},
		deleteFn: func(_ context.Context, b string) error {
			callLog = append(callLog, fmt.Sprintf("delete %s", b))
			return nil
		},
	}

	resolverCalled := 0
	resolver := &fakeResolver{
		resolveFn: func(_ context.Context, b string, cause error) error {
			resolverCalled++
			if b != branch {
				t.Fatalf("resolver branch = %q, want %q", b, branch)
			}
			var conflictErr *specmerge.ConflictError
			if !errors.As(cause, &conflictErr) {
				t.Fatalf("resolve cause = %v, want ConflictError", cause)
			}
			callLog = append(callLog, "resolve")
			return nil
		},
	}

	deps := specmerge.FinalizeDependencies{
		Git:              git,
		ConflictResolver: resolver,
	}

	if err := specmerge.FinalizeSpecBranch(ctx, deps, branch); err != nil {
		t.Fatalf("FinalizeSpecBranch returned %v", err)
	}
	if resolverCalled != 1 {
		t.Fatalf("resolver called %d times, want 1", resolverCalled)
	}

	want := []string{
		fmt.Sprintf("rebase %s %s", branch, config.DefaultBaseBranch),
		"merge 1 gromit/spec-payments",
		"resolve",
		"merge 2 gromit/spec-payments",
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

type fakeResolver struct {
	resolveFn func(ctx context.Context, branch string, cause error) error
}

func (f *fakeResolver) Resolve(ctx context.Context, branch string, cause error) error {
	if f.resolveFn == nil {
		return nil
	}
	return f.resolveFn(ctx, branch, cause)
}

type fakeManager struct {
	advanceFn func(ctx context.Context, branch, stage string) error
}

func (f *fakeManager) Advance(ctx context.Context, branch, stage string) error {
	if f.advanceFn == nil {
		return nil
	}
	return f.advanceFn(ctx, branch, stage)
}

// TestFinalizeSpecBranch_SpecbranchConflictErrorDetectedAfterConsolidation verifies that
// when specbranch.GitOps returns a ConflictError, specmerge's FinalizeSpecBranch
// properly detects it and calls the resolver. After ConflictError is consolidated
// into the specmerge package and imported by specbranch, both packages use the
// same type, ensuring errors.As can match the types correctly.
func TestFinalizeSpecBranch_SpecbranchConflictErrorDetectedAfterConsolidation(t *testing.T) {
	ctx := context.Background()
	branch := "gromit/spec-payments"
	defer withDefaultBaseBranch(t, "test-main-branch")()
	callLog := []string{}
	rebaseAttempts := 0

	// This fakeGitOps returns the consolidated ConflictError type (from specmerge)
	// to simulate what happens when specbranch.GitOps is wired as the real implementation.
	// Now that ConflictError is in specmerge and imported by specbranch,
	// this is the same type that specmerge.FinalizeSpecBranch expects and can detect.
	git := &fakeGitOps{
		rebaseFn: func(_ context.Context, b, onto string) error {
			rebaseAttempts++
			callLog = append(callLog, fmt.Sprintf("rebase %d %s", rebaseAttempts, onto))
			if rebaseAttempts == 1 {
				return &specmerge.ConflictError{
					Operation: "rebase",
					Err:       fmt.Errorf("merge conflict"),
				}
			}
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

	resolverCalled := 0
	resolver := &fakeResolver{
		resolveFn: func(_ context.Context, b string, cause error) error {
			resolverCalled++
			if b != branch {
				t.Fatalf("resolver branch = %q, want %q", b, branch)
			}
			var conflictErr *specmerge.ConflictError
			if !errors.As(cause, &conflictErr) {
				t.Fatalf("resolve cause = %v, want ConflictError", cause)
			}
			callLog = append(callLog, "resolve")
			return nil
		},
	}

	deps := specmerge.FinalizeDependencies{
		Git:              git,
		ConflictResolver: resolver,
	}

	// After consolidation, FinalizeSpecBranch should properly detect the ConflictError
	// returned by specbranch.GitOps and call the resolver
	if err := specmerge.FinalizeSpecBranch(ctx, deps, branch); err != nil {
		t.Fatalf("FinalizeSpecBranch returned %v", err)
	}

	if resolverCalled != 1 {
		t.Fatalf("resolver called %d times, want 1", resolverCalled)
	}

	want := []string{
		fmt.Sprintf("rebase 1 %s", config.DefaultBaseBranch),
		"resolve",
		fmt.Sprintf("rebase 2 %s", config.DefaultBaseBranch),
		"merge gromit/spec-payments",
		"delete gromit/spec-payments",
	}
	if !reflect.DeepEqual(callLog, want) {
		t.Fatalf("call order = %v, want %v", callLog, want)
	}
}

func TestFinalizeSpecBranch_ManagerAdvanceOnSuccess(t *testing.T) {
	ctx := context.Background()
	branch := "gromit/spec-manager-advance-success"
	defer withDefaultBaseBranch(t, "test-main-branch")()
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

	manager := &fakeManager{
		advanceFn: func(_ context.Context, b, stage string) error {
			callLog = append(callLog, fmt.Sprintf("advance %s %s", b, stage))
			if b != branch {
				t.Fatalf("advance branch = %q, want %q", b, branch)
			}
			if stage != specmerge.StageDone {
				t.Fatalf("advance stage = %q, want %q", stage, specmerge.StageDone)
			}
			return nil
		},
	}

	deps := specmerge.FinalizeDependencies{
		Git:     git,
		Manager: manager,
	}

	if err := specmerge.FinalizeSpecBranch(ctx, deps, branch); err != nil {
		t.Fatalf("FinalizeSpecBranch returned %v", err)
	}

	want := []string{
		fmt.Sprintf("rebase %s %s", branch, config.DefaultBaseBranch),
		fmt.Sprintf("merge %s", branch),
		fmt.Sprintf("delete %s", branch),
		fmt.Sprintf("advance %s %s", branch, specmerge.StageDone),
	}
	if !reflect.DeepEqual(callLog, want) {
		t.Fatalf("call order = %v, want %v", callLog, want)
	}
}

func TestFinalizeSpecBranch_ManagerNotAdvancedOnError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	branch := "gromit/spec-manager-advance-error"

	git := &fakeGitOps{
		rebaseFn: func(_ context.Context, b, onto string) error {
			return fmt.Errorf("rebase err %s", onto)
		},
	}

	manager := &fakeManager{
		advanceFn: func(_ context.Context, b, stage string) error {
			t.Fatalf("advance should not be called, but got %q %q", b, stage)
			return nil
		},
	}

	deps := specmerge.FinalizeDependencies{
		Git:     git,
		Manager: manager,
	}

	if err := specmerge.FinalizeSpecBranch(ctx, deps, branch); err == nil {
		t.Fatal("FinalizeSpecBranch expected error")
	}
}
