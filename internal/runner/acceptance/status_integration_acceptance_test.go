//go:build acceptance

package acceptance_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
)

// TestOrchestratorHelper_StatusIntegrationIdleWithHistory tests full status output
// when idle with a completed run history, backlog, and state files.
func TestOrchestratorHelper_StatusIntegrationIdleWithHistory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(gromitDir, "specs"),
			Plans: filepath.Join(gromitDir, "plans"),
		},
	}

	// Create backlog.jsonl
	if err := os.WriteFile(filepath.Join(gromitDir, "backlog.jsonl"), []byte(`{"id":"idea-1","text":"Idea one"}`), 0644); err != nil {
		t.Fatalf("Failed to write backlog: %v", err)
	}

	// Create a completed status file (running: false) from 3 hours ago
	sw, _ := runner.NewStatusWriter(gromitDir)
	sw.SetStartTime(time.Now().Add(-3 * time.Hour))
	if err := sw.WriteFinal(25); err != nil {
		t.Fatalf("Failed to write final status: %v", err)
	}

	// Create state.json with never-run retro
	if err := os.WriteFile(filepath.Join(gromitDir, "state.json"), []byte(`{"iterations_since_review": 10}`), 0644); err != nil {
		t.Fatalf("Failed to write state.json: %v", err)
	}

	var buf strings.Builder
	if err := runner.PrintStatus(gromitDir, cfg, &buf, nil, false); err != nil {
		t.Fatalf("PrintStatus() failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("Expected Pipeline section, got: %s", output)
	}
	if !strings.Contains(output, "Run: not running") {
		t.Errorf("Expected 'Run: not running', got: %s", output)
	}
	if !strings.Contains(output, "Last run:") {
		t.Errorf("Expected 'Last run:' info, got: %s", output)
	}
	if !strings.Contains(output, "25 iterations completed") {
		t.Errorf("Expected iteration count, got: %s", output)
	}
	if !strings.Contains(output, "ago") {
		t.Errorf("Expected relative time, got: %s", output)
	}
	if !strings.Contains(output, "Health:") {
		t.Errorf("Expected Health section, got: %s", output)
	}
	if !strings.Contains(output, "Last retro:  never") {
		t.Errorf("Expected 'Last retro: never', got: %s", output)
	}
	if !strings.Contains(output, "Last review: 10 iterations ago") {
		t.Errorf("Expected last review count, got: %s", output)
	}
	if !strings.Contains(output, "Next action:") {
		t.Errorf("Expected recommendation section, got: %s", output)
	}
}

// TestOrchestratorHelper_IntegrationQueueSectionIsStable verifies that the
// Integration Queue section appears consistently in status output across multiple calls,
// ensuring stable formatting and reliable queue display.
func TestOrchestratorHelper_IntegrationQueueSectionIsStable(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(gromitDir, "specs"),
			Plans: filepath.Join(gromitDir, "plans"),
		},
	}

	// Create a queue with entries in different states
	queueData := map[string]interface{}{
		"schema_version": 1,
		"updated_at":     "2026-02-28T12:00:00Z",
		"entries": []map[string]interface{}{
			{
				"branch":                 "gromit/feature-a",
				"session_id":             "session-a",
				"origin_command":         "review",
				"state":                  "ready",
				"lane":                   "code_lane",
				"created_at":             "2026-02-28T00:00:00Z",
				"updated_at":             "2026-02-28T12:00:00Z",
				"attempt_count":          1,
				"retry_count":            0,
				"fifo_seq":               1,
				"base_ref":               "origin/main",
				"head_sha":               "abc123",
				"changed_files_hash":     "sha256:hash",
				"last_error_code":        "",
				"last_error_message":     "",
				"last_transition_reason": "session_committed",
			},
			{
				"branch":                 "gromit/feature-b",
				"session_id":             "session-b",
				"origin_command":         "review",
				"state":                  "integrating",
				"lane":                   "safe_lane",
				"created_at":             "2026-02-28T00:00:00Z",
				"updated_at":             "2026-02-28T12:00:00Z",
				"attempt_count":          1,
				"retry_count":            0,
				"fifo_seq":               2,
				"base_ref":               "origin/main",
				"head_sha":               "def456",
				"changed_files_hash":     "sha256:hash2",
				"last_error_code":        "",
				"last_error_message":     "",
				"last_transition_reason": "gate_passed",
			},
		},
	}
	queueBytes, err := json.Marshal(queueData)
	if err != nil {
		t.Fatalf("Failed to marshal queue data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "integration-queue.json"), queueBytes, 0644); err != nil {
		t.Fatalf("Failed to write queue file: %v", err)
	}

	// Call PrintStatus multiple times
	outputs := make([]string, 3)
	for i := 0; i < 3; i++ {
		var buf strings.Builder
		if err := runner.PrintStatus(gromitDir, cfg, &buf, nil, false); err != nil {
			t.Fatalf("PrintStatus call %d failed: %v", i+1, err)
		}
		outputs[i] = buf.String()
	}

	// Verify Integration Queue section header is present in all calls
	for i, output := range outputs {
		if !strings.Contains(output, "Integration Queue:") {
			t.Errorf("Call %d missing 'Integration Queue:' header; got:\n%s", i+1, output)
		}
	}

	// Verify queue length is stable
	for i, output := range outputs {
		if !strings.Contains(output, "Queue length: 2") {
			t.Errorf("Call %d missing 'Queue length: 2'; got:\n%s", i+1, output)
		}
	}

	// Verify state counts are stable
	for i, output := range outputs {
		if !strings.Contains(output, "Ready: 1 | Integrating: 1") {
			t.Errorf("Call %d missing 'Ready: 1 | Integrating: 1'; got:\n%s", i+1, output)
		}
	}

	// Verify entries are consistently displayed
	for i, output := range outputs {
		if !strings.Contains(output, "gromit/feature-a") {
			t.Errorf("Call %d missing entry 'gromit/feature-a'; got:\n%s", i+1, output)
		}
		if !strings.Contains(output, "gromit/feature-b") {
			t.Errorf("Call %d missing entry 'gromit/feature-b'; got:\n%s", i+1, output)
		}
	}
}

// TestOrchestratorHelper_IntegrationQueueSectionDeterministic ensures the Integration Queue
// block is identical across multiple status invocations, making the CLI display predictable.
func TestOrchestratorHelper_IntegrationQueueSectionDeterministic(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(gromitDir, "specs"),
			Plans: filepath.Join(gromitDir, "plans"),
		},
	}

	queueData := map[string]interface{}{
		"schema_version": 1,
		"updated_at":     "2026-02-28T12:00:00Z",
		"entries": []map[string]interface{}{
			{
				"branch":                 "gromit/feature-c",
				"session_id":             "session-c",
				"origin_command":         "review",
				"state":                  "ready",
				"lane":                   "safe_lane",
				"created_at":             "2026-02-28T00:00:00Z",
				"updated_at":             "2026-02-28T12:00:00Z",
				"attempt_count":          1,
				"retry_count":            0,
				"fifo_seq":               1,
				"base_ref":               "origin/main",
				"head_sha":               "abc123",
				"changed_files_hash":     "sha256:hash",
				"last_error_code":        "",
				"last_error_message":     "",
				"last_transition_reason": "session_committed",
			},
			{
				"branch":                 "gromit/feature-d",
				"session_id":             "session-d",
				"origin_command":         "review",
				"state":                  "integrating",
				"lane":                   "code_lane",
				"created_at":             "2026-02-28T00:00:00Z",
				"updated_at":             "2026-02-28T12:00:00Z",
				"attempt_count":          1,
				"retry_count":            0,
				"fifo_seq":               2,
				"base_ref":               "origin/main",
				"head_sha":               "def456",
				"changed_files_hash":     "sha256:hash2",
				"last_error_code":        "",
				"last_error_message":     "",
				"last_transition_reason": "gate_passed",
			},
		},
	}
	queueBytes, err := json.Marshal(queueData)
	if err != nil {
		t.Fatalf("Failed to marshal queue data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "integration-queue.json"), queueBytes, 0644); err != nil {
		t.Fatalf("Failed to write queue file: %v", err)
	}

	outputs := captureStatusOutputs(t, gromitDir, cfg, 3)
	assertIntegrationQueueSectionStable(t, outputs)
}

// TestOrchestratorHelper_InvalidQueueSchemaInJSONOutput verifies that an invalid
// integration-queue.json schema surfaces queue_schema_invalid error in JSON output
// from the status command.
func TestOrchestratorHelper_InvalidQueueSchemaInJSONOutput(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(gromitDir, "specs"),
			Plans: filepath.Join(gromitDir, "plans"),
		},
	}

	// Create invalid integration-queue.json (corrupt/incomplete JSON)
	invalidQueueContent := `{"schema_version": 1, "entries": [{"branch": "invalid"`
	if err := os.WriteFile(filepath.Join(gromitDir, "integration-queue.json"), []byte(invalidQueueContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid queue: %v", err)
	}

	// Build status JSON to check for queue_schema_invalid error
	statusJSON, err := runner.BuildStatusJSON(gromitDir, cfg)
	if err != nil {
		t.Fatalf("BuildStatusJSON() failed: %v", err)
	}

	// Verify queue_schema_invalid error is in the Errors array
	found := false
	for _, errCode := range statusJSON.Errors {
		if errCode == "queue_schema_invalid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'queue_schema_invalid' in JSON errors, got: %v", statusJSON.Errors)
	}
}

// TestOrchestratorHelper_IntegrationQueueSectionBitIdenticalAcrossInvocations
// ensures that the exact formatted output of the Integration Queue section
// is identical across multiple status invocations (bit-for-bit comparison).
func TestOrchestratorHelper_IntegrationQueueSectionBitIdenticalAcrossInvocations(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(gromitDir, "specs"),
			Plans: filepath.Join(gromitDir, "plans"),
		},
	}

	queueData := map[string]interface{}{
		"schema_version": 1,
		"updated_at":     "2026-02-28T12:00:00Z",
		"entries": []map[string]interface{}{
			{
				"branch":                 "gromit/stable-feature",
				"session_id":             "session-stable",
				"origin_command":         "review",
				"state":                  "ready",
				"lane":                   "code_lane",
				"created_at":             "2026-02-28T00:00:00Z",
				"updated_at":             "2026-02-28T12:00:00Z",
				"attempt_count":          1,
				"retry_count":            0,
				"fifo_seq":               1,
				"base_ref":               "origin/main",
				"head_sha":               "stable123",
				"changed_files_hash":     "sha256:hash",
				"last_error_code":        "",
				"last_error_message":     "",
				"last_transition_reason": "session_committed",
			},
		},
	}
	queueBytes, err := json.Marshal(queueData)
	if err != nil {
		t.Fatalf("Failed to marshal queue data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "integration-queue.json"), queueBytes, 0644); err != nil {
		t.Fatalf("Failed to write queue file: %v", err)
	}

	// Capture status outputs and extract Integration Queue sections
	outputs := captureStatusOutputs(t, gromitDir, cfg, 4)
	sections := make([]string, len(outputs))
	for i, output := range outputs {
		sections[i] = extractIntegrationQueueSection(output)
	}

	// Verify all sections are identical (bit-for-bit)
	if len(sections) == 0 {
		t.Fatalf("No outputs captured")
	}
	reference := sections[0]
	for i := 1; i < len(sections); i++ {
		if sections[i] != reference {
			t.Errorf("Integration Queue section differs at invocation %d\nExpected:\n%s\nGot:\n%s", i+1, reference, sections[i])
		}
	}
}
