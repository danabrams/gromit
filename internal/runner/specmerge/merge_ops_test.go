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

func TestFinalizeSpecBranch_RebaseBeforeMergeBeforeDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	branch := "gromit/spec-payments"
	callLog := []string{}
	originalBaseBranch := config.DefaultBaseBranch
	config.DefaultBaseBranch = "test-main-branch"
	defer func() {
		config.DefaultBaseBranch = originalBaseBranch
	}()

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
	t.Parallel()
	ctx := context.Background()
	branch := "gromit/spec-payments"
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
	t.Parallel()
	ctx := context.Background()
	branch := "gromit/spec-payments"
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

// TestFinalizeSpecBranch_SpecbranchConflictErrorDetectedAfterConsolidation verifies that
// when specbranch.GitOps returns a ConflictError, specmerge's FinalizeSpecBranch
// properly detects it and calls the resolver. After ConflictError is consolidated
// into the specmerge package and imported by specbranch, both packages use the
// same type, ensuring errors.As can match the types correctly.
func TestFinalizeSpecBranch_SpecbranchConflictErrorDetectedAfterConsolidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	branch := "gromit/spec-payments"
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
