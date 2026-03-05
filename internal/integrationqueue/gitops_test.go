package integrationqueue

import (
	"context"
	"errors"
	"testing"
)

// TestFetchAndRebaseErrorPropagation verifies that FetchAndRebase errors are properly propagated.
func TestFetchAndRebaseErrorPropagation(t *testing.T) {
	ctx := context.Background()
	entry := Entry{Branch: "test-branch", SessionID: "test"}

	wantErr := errors.New("fetch failed")
	mock := NewErrorMockGitOps(wantErr, nil, nil, nil)

	err := mock.FetchAndRebase(ctx, entry)
	if err != wantErr {
		t.Fatalf("FetchAndRebase() error = %v, want %v", err, wantErr)
	}
}

// NewErrorMockGitOps creates a mock GitOps implementation that returns specified errors.
// This function does not exist yet and will cause compilation to fail in RED phase.
