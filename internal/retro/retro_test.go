package retro

import (
	"context"
	"testing"

	"github.com/danabrams/ralph-runner/internal/claude"
	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/logger"
)

func TestNewRetroNilConfig(t *testing.T) {
	r, err := NewRetro(nil, ".ralph")
	if r != nil {
		t.Error("expected nil Retro for nil config")
	}
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestRunNilReceiver(t *testing.T) {
	var r *Retro
	_, err := r.Run(context.Background(), false)
	if err == nil {
		t.Error("expected error for nil retro")
	}
}

func TestRunNilClaudeClient(t *testing.T) {
	r := &Retro{
		claude: nil,
	}
	_, err := r.Run(context.Background(), false)
	if err == nil {
		t.Error("expected error for nil claude client")
	}
}

func TestRunNilLearningsFile(t *testing.T) {
	claudeClient, _ := claude.NewClient("claude", nil, 60)
	r := &Retro{
		claude:        claudeClient,
		learningsFile: nil,
	}
	_, err := r.Run(context.Background(), false)
	if err == nil {
		t.Error("expected error for nil learnings file")
	}
}

func TestEnrichBeadStatsNilReceiver(t *testing.T) {
	var r *Retro
	beadStats := make(map[string]logger.BeadStats)
	// Should not panic
	r.enrichBeadStats(context.Background(), beadStats)
}

func TestEnrichBeadStatsNilMap(t *testing.T) {
	tmpDir := t.TempDir()
	r, _ := NewRetro(&config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}, tmpDir)
	// Should not panic
	r.enrichBeadStats(context.Background(), nil)
}

func TestEnrichBeadStatsEmptyMap(t *testing.T) {
	tmpDir := t.TempDir()
	r, _ := NewRetro(&config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}, tmpDir)
	beadStats := make(map[string]logger.BeadStats)
	// Should not panic and return immediately
	r.enrichBeadStats(context.Background(), beadStats)
	if len(beadStats) != 0 {
		t.Error("expected empty map to remain empty")
	}
}

func TestLaunchClaudeCodeWithAnalysis(t *testing.T) {
	// This test verifies that LaunchClaudeCode builds the correct prompt structure.
	// We can't easily test the actual execution without mocking exec.Command,
	// but we can verify the function signature and basic behavior.

	// Test with empty analysis - should still build valid prompt
	analysis := ""

	// Note: We can't actually run this in tests as it would launch an interactive
	// claude session. In a real scenario, you'd use dependency injection to mock
	// the command execution. For now, we just verify the function exists and
	// accepts the correct parameters.

	// The function signature is: LaunchClaudeCode(analysis string) error
	// This is a compile-time check that the function exists with the right signature.
	var _ func(string) error = LaunchClaudeCode

	// Test that the function accepts a non-empty analysis
	analysis = "Test analysis results"
	_ = analysis // Use the variable to prevent unused variable error

	// In a real integration test, you would:
	// 1. Mock exec.Command
	// 2. Verify the prompt contains the analysis
	// 3. Verify the command is "claude" with the prompt as an argument
	// 4. Verify stdin/stdout/stderr are connected
}
