package runner

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestComputeScopedTestCommand_EmptyPackages(t *testing.T) {
	result := computeScopedTestCommand(nil)
	if result != "" {
		t.Errorf("expected empty string for nil packages, got %q", result)
	}

	result = computeScopedTestCommand([]string{})
	if result != "" {
		t.Errorf("expected empty string for empty packages, got %q", result)
	}
}

func TestComputeScopedTestCommand_SinglePackage(t *testing.T) {
	result := computeScopedTestCommand([]string{"internal/runner"})
	if !strings.Contains(result, "go test") {
		t.Errorf("expected 'go test' in result, got %q", result)
	}
	if !strings.Contains(result, "./internal/runner/...") {
		t.Errorf("expected './internal/runner/...' in result, got %q", result)
	}
}

func TestComputeScopedTestCommand_MultiplePackages(t *testing.T) {
	result := computeScopedTestCommand([]string{"internal/runner", "internal/config"})
	if !strings.Contains(result, "./internal/runner/...") {
		t.Errorf("expected './internal/runner/...' in result, got %q", result)
	}
	if !strings.Contains(result, "./internal/config/...") {
		t.Errorf("expected './internal/config/...' in result, got %q", result)
	}
}

func TestComputeScopedTestCommand_RootPackage(t *testing.T) {
	result := computeScopedTestCommand([]string{"."})
	if result != "go test ./..." {
		t.Errorf("expected 'go test ./...', got %q", result)
	}
}

func TestComputeScopedTestCommand_RootAndSubpackage(t *testing.T) {
	result := computeScopedTestCommand([]string{".", "internal/runner"})
	if result != "go test ./... ./internal/runner/..." {
		t.Errorf("expected 'go test ./... ./internal/runner/...', got %q", result)
	}
}

// TestInjectScopedTestCommand_PopulatesPromptContextFromTouchedPackages verifies that
// injectScopedTestCommand sets bc.PromptCtx.ScopedTestCommand from bc.TouchedPackages.
func TestInjectScopedTestCommand_PopulatesPromptContextFromTouchedPackages(t *testing.T) {
	bc := &runtypes.BeadContext{
		TouchedPackages: []string{"internal/runner", "internal/config"},
		PromptCtx:       &prompt.Context{},
	}

	injectScopedTestCommand(bc)

	want := "go test ./internal/runner/... ./internal/config/..."
	if bc.PromptCtx.ScopedTestCommand != want {
		t.Errorf("ScopedTestCommand = %q, want %q", bc.PromptCtx.ScopedTestCommand, want)
	}
}

// TestInjectScopedTestCommand_ClearsCommandWhenNoPackages verifies that
// injectScopedTestCommand clears the command when there are no touched packages.
func TestInjectScopedTestCommand_ClearsCommandWhenNoPackages(t *testing.T) {
	bc := &runtypes.BeadContext{
		TouchedPackages: nil,
		PromptCtx:       &prompt.Context{ScopedTestCommand: "go test ./old/..."},
	}

	injectScopedTestCommand(bc)

	if bc.PromptCtx.ScopedTestCommand != "" {
		t.Errorf("ScopedTestCommand = %q, want empty string", bc.PromptCtx.ScopedTestCommand)
	}
}

// TestInjectScopedTestCommand_NilPromptCtx verifies that injectScopedTestCommand
// does not panic when PromptCtx is nil.
func TestInjectScopedTestCommand_NilPromptCtx(t *testing.T) {
	bc := &runtypes.BeadContext{
		TouchedPackages: []string{"internal/runner"},
		PromptCtx:       nil,
	}

	// Should not panic
	injectScopedTestCommand(bc)
}
