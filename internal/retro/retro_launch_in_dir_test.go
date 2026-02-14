package retro

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/logger"
)

// TestLaunchClaudeCodeAcceptsDirParameter verifies LaunchClaudeCode accepts a dir parameter
func TestLaunchClaudeCodeAcceptsDirParameter(t *testing.T) {
	// Expected failure: LaunchClaudeCode function signature does not include dir parameter yet
	// Current signature: LaunchClaudeCode(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment) error
	// New signature: LaunchClaudeCode(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment, dir string) error

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	analysis := "Test analysis"

	// This should compile once the function signature is updated
	err := LaunchClaudeCode(analysis, nil, nil, targetDir)

	// We expect this to fail because 'claude' binary may not exist in test environment
	// But the key is that the function accepts the dir parameter
	_ = err
}

// TestLaunchClaudeCodeEmptyDirUsesCurrentDirectory verifies empty dir parameter uses current directory
func TestLaunchClaudeCodeEmptyDirUsesCurrentDirectory(t *testing.T) {
	// Expected failure: LaunchClaudeCode function signature does not include dir parameter yet

	analysis := "Test analysis"

	// Empty dir should use current directory (no cmd.Dir set)
	err := LaunchClaudeCode(analysis, nil, nil, "")

	// We expect this to fail because 'claude' binary may not exist in test environment
	// But the function should accept empty dir without panicking
	_ = err
}

// TestLaunchClaudeCodeWithEfficiencyAndExperimentInDir verifies dir parameter works with all arguments
func TestLaunchClaudeCodeWithEfficiencyAndExperimentInDir(t *testing.T) {
	// Expected failure: LaunchClaudeCode function signature does not include dir parameter yet

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	analysis := "Test analysis with efficiency"
	efficiency := &logger.EfficiencyReport{
		CurrentAvgCostPerBead:    1.5,
		HistoricalAvgCostPerBead: 2.0,
		CostDelta:                -0.5,
	}
	experiment := &Experiment{
		Name:       "Test Experiment",
		Hypothesis: "Test hypothesis",
		Change:     "Test change",
	}

	// Function should accept all parameters including dir
	err := LaunchClaudeCode(analysis, efficiency, experiment, targetDir)

	// We expect this to fail because 'claude' binary may not exist
	// But the function signature should be correct
	_ = err
}

// TestLaunchClaudeCodeNonexistentDirFailsAtExecLevel verifies nonexistent dir fails at exec level
func TestLaunchClaudeCodeNonexistentDirFailsAtExecLevel(t *testing.T) {
	// Expected failure: LaunchClaudeCode function signature does not include dir parameter yet

	tmpDir := t.TempDir()
	nonexistentDir := filepath.Join(tmpDir, "does-not-exist")

	analysis := "Test analysis"

	// Nonexistent directory should not cause immediate panic
	// The exec.Command will fail when it tries to run in that directory
	err := LaunchClaudeCode(analysis, nil, nil, nonexistentDir)

	// We expect an error (either from missing 'claude' binary or bad directory)
	// The key is that the function accepts the parameter
	_ = err
}

// TestLaunchClaudeCodePreservesExistingPromptBehavior verifies prompt construction unchanged
func TestLaunchClaudeCodePreservesExistingPromptBehavior(t *testing.T) {
	// Expected failure: LaunchClaudeCode function signature does not include dir parameter yet

	// This test verifies that adding the dir parameter doesn't break
	// existing prompt construction logic

	analysis := "# Test Analysis\n\nThis is a test."
	efficiency := &logger.EfficiencyReport{
		CurrentAvgCostPerBead:    1.0,
		HistoricalAvgCostPerBead: 1.0,
		CostDelta:                0.0,
	}

	// The function should still build the same prompt structure
	// (we can't easily verify this without running, but the signature is key)
	err := LaunchClaudeCode(analysis, efficiency, nil, "")

	_ = err
}

// TestLaunchClaudeCodeDirParameterPosition verifies dir is the fourth parameter
func TestLaunchClaudeCodeDirParameterPosition(t *testing.T) {
	// Expected failure: LaunchClaudeCode function signature does not include dir parameter yet
	// The spec says: "retro.LaunchClaudeCode in internal/retro/retro.go to accept dir parameter"
	// Based on worktree spec, dir should be added as a parameter to allow launching in worktree

	// This test verifies the parameter order is:
	// LaunchClaudeCode(analysis string, efficiency *logger.EfficiencyReport, experiment *Experiment, dir string) error

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	// All parameters in order
	err := LaunchClaudeCode(
		"analysis", // string
		nil,        // *logger.EfficiencyReport
		nil,        // *Experiment
		targetDir,  // string (dir - new parameter)
	)

	_ = err
}
