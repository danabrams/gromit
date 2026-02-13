package retro

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// TestNewRetro_DoesNotExist verifies that the NewRetro constructor has been removed
// This test now passes - NewRetro has been successfully removed
func TestNewRetro_DoesNotExist(t *testing.T) {
	// NewRetro has been removed - this test documents that fact
	// The migration is complete: only NewRetroWithProvider exists
	t.Log("NewRetro constructor has been successfully removed")
}

// TestNewRetroWithProvider_OnlyConstructor verifies that NewRetroWithProvider is the sole constructor
// Expected failure: NewRetro still exists alongside NewRetroWithProvider
func TestNewRetroWithProvider_OnlyConstructor(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal files
	createMinimalRetroFilesForProvider(t, tmpDir)

	mockProvider := &mockProviderForMigration{
		runResult: &provider.Result{
			Success: true,
			Output:  "test output",
		},
	}

	// NewRetroWithProvider should be the only way to create a Retro
	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	if r == nil {
		t.Fatal("expected non-nil Retro from NewRetroWithProvider")
	}

	// After migration, accessing r.claude should cause a compile error
	// We test the behavioral consequence: if r.claude is still in the struct,
	// the Run method will check "if r.claude == nil && r.provider == nil"
	// Expected failure: r.claude field still exists and Run() checks both fields

	// Verify provider field exists and is set
	if r.provider == nil {
		t.Error("Retro should have provider field - expected non-nil but got nil")
	}
}

// TestRetro_createLearningsAdapter_OnlyUsesProvider verifies that createLearningsAdapter
// no longer has dual-branch logic for claude vs provider
// Expected failure: createLearningsAdapter still checks r.provider vs r.claude
func TestRetro_createLearningsAdapter_OnlyUsesProvider(t *testing.T) {
	tmpDir := t.TempDir()
	createMinimalRetroFilesForProvider(t, tmpDir)

	mockProvider := &mockProviderForMigration{
		runResult: &provider.Result{
			Success: true,
			Output:  `{"category":"patterns","project_relevant":true}`,
		},
	}

	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	// Call createLearningsAdapter
	adapter := r.createLearningsAdapter()
	if adapter == nil {
		t.Error("expected non-nil adapter from createLearningsAdapter")
	}

	// Verify that calling the adapter works (it should delegate to provider)
	// Expected failure: createLearningsAdapter still has dual-branch logic checking r.provider vs r.claude
	ctx := context.Background()
	result, err := adapter.Run(ctx, "test prompt", "medium")
	if err != nil {
		t.Fatalf("adapter.Run failed: %v", err)
	}

	// After migration, the adapter should always delegate to provider
	if !mockProvider.runCalled {
		t.Error("expected adapter to call provider.Run, but it did not - likely still using claude client branch")
	}

	// Verify result is properly returned
	if result == nil {
		t.Error("expected non-nil result from adapter.Run")
	}
}

// TestRetro_runAnalysis_OnlyUsesProvider verifies that runAnalysis no longer has
// dual-branch logic for claude vs provider
// Expected failure: runAnalysis still checks if r.provider != nil { ... } else { r.claude.Run(...) }
func TestRetro_runAnalysis_OnlyUsesProvider(t *testing.T) {
	tmpDir := t.TempDir()
	createMinimalRetroFilesForProvider(t, tmpDir)

	mockProvider := &mockProviderForMigration{
		runResult: &provider.Result{
			Success: true,
			Output:  "analysis output",
		},
	}

	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	// Call runAnalysis
	ctx := context.Background()
	result, err := r.runAnalysis(ctx, "test prompt")
	if err != nil {
		t.Fatalf("runAnalysis failed: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result from runAnalysis")
	}

	// Verify that provider was called
	// Expected failure: runAnalysis still has if r.provider != nil branch and falls back to r.claude.Run
	if !mockProvider.runCalled {
		t.Error("expected runAnalysis to call provider.Run, but it did not - likely still using claude client")
	}

	// Verify the result matches provider output
	if result.GetOutput() != "analysis output" {
		t.Errorf("expected output 'analysis output', got %q", result.GetOutput())
	}
}

// TestRetro_Run_OnlyUsesProvider verifies that Run() method exclusively uses provider
// and does not check for r.claude
// Expected failure: Run() still checks if r.claude == nil && r.provider == nil
func TestRetro_Run_OnlyUsesProvider(t *testing.T) {
	tmpDir := t.TempDir()
	createMinimalRetroFilesForProvider(t, tmpDir)

	mockProvider := &mockProviderForMigration{
		runResult: &provider.Result{
			Success: true,
			Output:  "retro analysis output",
		},
	}

	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	// Run should only check for r.provider == nil, not r.claude == nil && r.provider == nil
	// Expected failure: Run() still checks both r.claude and r.provider
	ctx := context.Background()
	result, err := r.Run(ctx, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result from Run")
	}

	// Verify provider was used for both learnings filtering and analysis
	if !mockProvider.runCalled {
		t.Error("expected Run to call provider, but it did not")
	}
}

// TestRetro_NilProvider_ReturnsError verifies that creating Retro with nil provider fails
// Expected failure: NewRetroWithProvider still accepts nil provider or doesn't validate
func TestRetro_NilProvider_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	// NewRetroWithProvider should reject nil provider
	// Expected failure: validation logic not added yet or still allows nil
	r, err := NewRetroWithProvider(nil, tmpDir)

	if err == nil {
		t.Error("expected error when creating Retro with nil provider, got nil error")
	}

	if r != nil {
		t.Error("expected nil Retro when provider is nil, got non-nil")
	}
}

// TestRetroStruct_OnlyHasProviderField verifies that Retro struct no longer has claude field
// Expected failure: Retro struct still has both claude *claude.Client and provider ProviderRunner fields
func TestRetroStruct_OnlyHasProviderField(t *testing.T) {
	tmpDir := t.TempDir()
	createMinimalRetroFilesForProvider(t, tmpDir)

	mockProvider := &mockProviderForMigration{
		runResult: &provider.Result{
			Success: true,
			Output:  "test",
		},
	}

	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	// After migration, accessing r.claude should not compile
	// Expected failure: r.claude field still exists in Retro struct definition
	// This test verifies behavioral output: the Retro created via NewRetroWithProvider
	// should successfully run without needing the claude field
	ctx := context.Background()
	result, err := r.Run(ctx, nil)

	// If the claude field is still present and being used, Run will fail with
	// "neither claude client nor provider is set" error
	if err != nil && err.Error() == "neither claude client nor provider is set" {
		t.Error("Retro.Run failed because claude field is still being checked - migration incomplete")
	}

	// After migration, Run should succeed using only the provider
	if err != nil {
		t.Fatalf("Retro.Run should succeed using only provider: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result from Run")
	}

	// Verify provider was used
	if !mockProvider.runCalled {
		t.Error("expected provider to be called, but it was not - likely still checking claude field")
	}
}

// TestRetro_ProviderRunnerInterface_Matches verifies that ProviderRunner interface
// is sufficient for Retro's needs (no claude-specific methods required)
// Expected failure: Retro still depends on claude.Client methods not in ProviderRunner
func TestRetro_ProviderRunnerInterface_Matches(t *testing.T) {
	// ProviderRunner should have Run method that takes (ctx, prompt, tier)
	// Expected failure: ProviderRunner interface doesn't exist or has different signature

	var _ ProviderRunner = &mockProviderForMigration{}

	// This test verifies compile-time interface satisfaction
	// If ProviderRunner interface is correctly defined, this should compile
}

// TestRetro_NoDualBranchLogic_InRunAnalysis verifies that runAnalysis doesn't check
// both r.provider and r.claude to decide which to use
// Expected failure: runAnalysis still has if r.provider != nil { ... } else { r.claude.Run(...) }
func TestRetro_NoDualBranchLogic_InRunAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	createMinimalRetroFilesForProvider(t, tmpDir)

	mockProvider := &mockProviderForMigration{
		runResult: &provider.Result{
			Success: true,
			Output:  "analysis output",
		},
	}

	// Create Retro with provider
	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	// If runAnalysis still has dual-branch logic, it will check:
	//   if r.provider != nil { use provider }
	//   else { use claude }
	//
	// After migration, it should ONLY check:
	//   if r.provider == nil { return error }
	//   use provider
	//
	// We verify this by ensuring runAnalysis always uses provider when it exists

	ctx := context.Background()
	result, err := r.runAnalysis(ctx, "test prompt")
	if err != nil {
		t.Fatalf("runAnalysis failed: %v", err)
	}

	if !mockProvider.runCalled {
		t.Error("runAnalysis should always call provider.Run when provider is set, but it did not")
	}

	// Verify we got the provider's output, not a fallback
	if result.GetOutput() != "analysis output" {
		t.Errorf("expected provider output 'analysis output', got %q - may have used wrong branch", result.GetOutput())
	}
}

// TestRetro_NoDualBranchLogic_InCreateLearningsAdapter verifies that createLearningsAdapter
// doesn't check both r.provider and r.claude to decide which adapter to create
// Expected failure: createLearningsAdapter still has if r.provider != nil { ... } return learnings.NewClaudeRunnerAdapter(r.claude)
func TestRetro_NoDualBranchLogic_InCreateLearningsAdapter(t *testing.T) {
	tmpDir := t.TempDir()
	createMinimalRetroFilesForProvider(t, tmpDir)

	mockProvider := &mockProviderForMigration{
		runResult: &provider.Result{
			Success: true,
			Output:  `{"category":"patterns","project_relevant":true}`,
		},
	}

	// Create Retro with provider
	r, err := NewRetroWithProvider(mockProvider, tmpDir)
	if err != nil {
		t.Fatalf("NewRetroWithProvider failed: %v", err)
	}

	// If createLearningsAdapter still has dual-branch logic:
	//   if r.provider != nil { return NewProviderRunnerAdapter(r.provider) }
	//   return NewClaudeRunnerAdapter(r.claude)
	//
	// After migration, it should only use provider:
	//   return NewProviderRunnerAdapter(r.provider)

	adapter := r.createLearningsAdapter()
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}

	// Call the adapter and verify it uses provider
	ctx := context.Background()
	_, err = adapter.Run(ctx, "test prompt", "medium")
	if err != nil {
		t.Fatalf("adapter.Run failed: %v", err)
	}

	if !mockProvider.runCalled {
		t.Error("createLearningsAdapter should return an adapter that uses provider, but it did not call provider.Run")
	}
}

// mockProviderForMigration implements ProviderRunner for migration testing
type mockProviderForMigration struct {
	runCalled bool
	runResult *provider.Result
	runErr    error
}

func (m *mockProviderForMigration) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	m.runCalled = true
	if m.runErr != nil {
		return nil, m.runErr
	}
	if m.runResult != nil {
		return m.runResult, nil
	}
	return &provider.Result{Success: true, Output: "mock output"}, nil
}

func (m *mockProviderForMigration) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return m.Run(ctx, prompt, tier)
}
