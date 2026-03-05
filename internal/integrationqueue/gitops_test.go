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

// TestMergeToMainErrorPropagation verifies that MergeToMain errors are properly propagated.
func TestMergeToMainErrorPropagation(t *testing.T) {
	ctx := context.Background()
	entry := Entry{Branch: "test-branch", SessionID: "test"}

	wantErr := errors.New("merge conflict")
	mock := NewCallTrackingMockGitOps(nil, wantErr, nil, nil)

	err := mock.MergeToMain(ctx, entry)
	if err != wantErr {
		t.Fatalf("MergeToMain() error = %v, want %v", err, wantErr)
	}
}

// TestPushErrorPropagation verifies that Push errors are properly propagated.
func TestPushErrorPropagation(t *testing.T) {
	ctx := context.Background()

	wantErr := errors.New("push failed")
	mock := NewCallTrackingMockGitOps(nil, nil, wantErr, nil)

	err := mock.Push(ctx)
	if err != wantErr {
		t.Fatalf("Push() error = %v, want %v", err, wantErr)
	}
}

// NewErrorMockGitOps creates a mock GitOps implementation that returns specified errors.
func NewErrorMockGitOps(fetchErr, mergeErr, pushErr, cleanupErr error) GitOps {
	return &errorMockGitOps{
		fetchErr:   fetchErr,
		mergeErr:   mergeErr,
		pushErr:    pushErr,
		cleanupErr: cleanupErr,
	}
}

type errorMockGitOps struct {
	fetchErr   error
	mergeErr   error
	pushErr    error
	cleanupErr error
}

func (m *errorMockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	return m.fetchErr
}

func (m *errorMockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	return m.mergeErr
}

func (m *errorMockGitOps) Push(ctx context.Context) error {
	return m.pushErr
}

func (m *errorMockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	return m.cleanupErr
}

// NewCallTrackingMockGitOps creates a mock GitOps that tracks method calls and returns specified errors.
func NewCallTrackingMockGitOps(fetchErr, mergeErr, pushErr, cleanupErr error) GitOps {
	return &callTrackingMockGitOps{
		fetchErr:   fetchErr,
		mergeErr:   mergeErr,
		pushErr:    pushErr,
		cleanupErr: cleanupErr,
	}
}

type callTrackingMockGitOps struct {
	fetchErr   error
	mergeErr   error
	pushErr    error
	cleanupErr error
	calls      []string
}

func (m *callTrackingMockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "FetchAndRebase")
	return m.fetchErr
}

func (m *callTrackingMockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "MergeToMain")
	return m.mergeErr
}

func (m *callTrackingMockGitOps) Push(ctx context.Context) error {
	m.calls = append(m.calls, "Push")
	return m.pushErr
}

func (m *callTrackingMockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	m.calls = append(m.calls, "Cleanup")
	return m.cleanupErr
}
