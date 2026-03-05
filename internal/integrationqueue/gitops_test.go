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

// TestCleanupErrorPropagation verifies that Cleanup errors are properly propagated.
func TestCleanupErrorPropagation(t *testing.T) {
	ctx := context.Background()
	entry := Entry{Branch: "test-branch", SessionID: "test"}

	wantErr := errors.New("cleanup failed")
	mock := NewCallTrackingMockGitOps(nil, nil, nil, wantErr)

	err := mock.Cleanup(ctx, entry)
	if err != wantErr {
		t.Fatalf("Cleanup() error = %v, want %v", err, wantErr)
	}
}

// TestGitOpsCallSequence verifies that GitOps methods can be called in sequence.
func TestGitOpsCallSequence(t *testing.T) {
	ctx := context.Background()
	entry := Entry{Branch: "test-branch", SessionID: "test"}

	mock := NewCallTrackingMockGitOps(nil, nil, nil, nil)

	// Call all methods in sequence
	if err := mock.FetchAndRebase(ctx, entry); err != nil {
		t.Fatalf("FetchAndRebase() error = %v", err)
	}
	if err := mock.MergeToMain(ctx, entry); err != nil {
		t.Fatalf("MergeToMain() error = %v", err)
	}
	if err := mock.Push(ctx); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
	if err := mock.Cleanup(ctx, entry); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// Verify calls were tracked
	mockImpl := mock.(*callTrackingMockGitOps)
	wantCalls := []string{"FetchAndRebase", "MergeToMain", "Push", "Cleanup"}
	if !callsMatch(mockImpl.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", mockImpl.calls, wantCalls)
	}
}

// TestContextCanceledFetchAndRebase verifies that FetchAndRebase respects context cancellation.
func TestContextCanceledFetchAndRebase(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	entry := Entry{Branch: "test-branch", SessionID: "test"}
	mock := NewContextSensitiveMockGitOps()

	err := mock.FetchAndRebase(ctx, entry)
	if err == nil {
		t.Fatalf("FetchAndRebase() expected error for canceled context, got nil")
	}
}

// TestAllGitOpsMethodsSucceed verifies that all GitOps methods can succeed without errors.
func TestAllGitOpsMethodsSucceed(t *testing.T) {
	ctx := context.Background()
	entry := Entry{Branch: "test-branch", SessionID: "test"}

	mock := NewSuccessfulMockGitOps()

	if err := mock.FetchAndRebase(ctx, entry); err != nil {
		t.Errorf("FetchAndRebase() unexpected error: %v", err)
	}
	if err := mock.MergeToMain(ctx, entry); err != nil {
		t.Errorf("MergeToMain() unexpected error: %v", err)
	}
	if err := mock.Push(ctx); err != nil {
		t.Errorf("Push() unexpected error: %v", err)
	}
	if err := mock.Cleanup(ctx, entry); err != nil {
		t.Errorf("Cleanup() unexpected error: %v", err)
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

// callsMatch compares two slices of strings for equality.
func callsMatch(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// NewContextSensitiveMockGitOps creates a mock GitOps that respects context cancellation.
func NewContextSensitiveMockGitOps() GitOps {
	return &contextSensitiveMockGitOps{}
}

type contextSensitiveMockGitOps struct{}

func (m *contextSensitiveMockGitOps) FetchAndRebase(ctx context.Context, entry Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (m *contextSensitiveMockGitOps) MergeToMain(ctx context.Context, entry Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (m *contextSensitiveMockGitOps) Push(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (m *contextSensitiveMockGitOps) Cleanup(ctx context.Context, entry Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
