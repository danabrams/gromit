package fixtures_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestCodexStreamSuccessFixtureHasOrderedLifecycle verifies that
// codex_stream_success.jsonl contains a minimal but realistic ordered stream
// with start, delta, and end events — not just assistant + result.
func TestCodexStreamSuccessFixtureHasOrderedLifecycle(t *testing.T) {
	content := readFixtureFile(t, "codex_stream_success.jsonl")
	events := parseJSONLEvents(t, content)

	if len(events) < 3 {
		t.Fatalf("codex_stream_success.jsonl has %d JSON events, want at least 3 (start, delta/assistant, end)", len(events))
	}

	firstType := extractEventType(t, events[0])
	if !isStreamStartType(firstType) {
		t.Fatalf("first event type = %q, want a start event type (start, stream_start, thread.started, or response.started)", firstType)
	}

	hasDelta := false
	for _, event := range events[1 : len(events)-1] {
		if isStreamDeltaType(extractEventType(t, event)) {
			hasDelta = true
			break
		}
	}
	if !hasDelta {
		t.Fatal("codex_stream_success.jsonl must have at least one delta/assistant event between start and terminal events")
	}

	lastType := extractEventType(t, events[len(events)-1])
	if !isStreamEndType(lastType) {
		t.Fatalf("terminal event type = %q, want an end event type (end, stream_end, result, or response.completed)", lastType)
	}

	last := events[len(events)-1]
	resultStatus := nestedStringField(last, "result", "status")
	if resultStatus != "success" {
		t.Fatalf("terminal result.status = %q, want %q", resultStatus, "success")
	}
}

// TestCodexStreamFailureFixtureTerminatesWithError verifies that
// codex_stream_failure.jsonl contains a realistic stream that terminates with
// an explicit error event type — not just a result event with status "failure".
func TestCodexStreamFailureFixtureTerminatesWithError(t *testing.T) {
	content := readFixtureFile(t, "codex_stream_failure.jsonl")
	events := parseJSONLEvents(t, content)

	if len(events) < 3 {
		t.Fatalf("codex_stream_failure.jsonl has %d JSON events, want at least 3 (start, delta/assistant, error)", len(events))
	}

	firstType := extractEventType(t, events[0])
	if !isStreamStartType(firstType) {
		t.Fatalf("first event type = %q, want a start event type", firstType)
	}

	lastType := extractEventType(t, events[len(events)-1])
	if !isStreamErrorType(lastType) {
		t.Fatalf("terminal event type = %q, want an explicit error event type (error, stream_error, or response.error)", lastType)
	}

	last := events[len(events)-1]
	errorField, ok := last["error"]
	if !ok {
		t.Fatal("terminal error event must include an 'error' field")
	}
	errorStr, ok := errorField.(string)
	if !ok || strings.TrimSpace(errorStr) == "" {
		t.Fatal("terminal error event 'error' field must be a non-empty string")
	}
}

// TestCodexStreamFixturesHaveProvenanceAndRefreshComments verifies that each
// codex JSONL stream fixture includes both a provenance and refresh comment.
func TestCodexStreamFixturesHaveProvenanceAndRefreshComments(t *testing.T) {
	fixtures := []string{
		"codex_stream_success.jsonl",
		"codex_stream_failure.jsonl",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			content := readFixtureFile(t, name)
			commentLines := extractCommentLines(content)

			if len(commentLines) < 2 {
				t.Fatalf("fixture %q has %d comment lines, want at least 2 (# provenance: and # refresh:)", name, len(commentLines))
			}
			if !strings.HasPrefix(commentLines[0], "# provenance:") {
				t.Fatalf("first comment line = %q, want prefix %q", commentLines[0], "# provenance:")
			}
			if !strings.HasPrefix(commentLines[1], "# refresh:") {
				t.Fatalf("second comment line = %q, want prefix %q", commentLines[1], "# refresh:")
			}
		})
	}
}

// TestClaudeStreamSuccessFixtureHasCanonicalLifecycle verifies that
// claude_stream_success.jsonl uses a minimal deterministic stream lifecycle
// with start, delta/assistant, and terminal success events.
func TestClaudeStreamSuccessFixtureHasCanonicalLifecycle(t *testing.T) {
	content := readFixtureFile(t, "claude_stream_success.jsonl")
	events := parseJSONLEvents(t, content)

	if len(events) != 3 {
		t.Fatalf("claude_stream_success.jsonl has %d JSON events, want exactly 3 for deterministic snapshots", len(events))
	}

	firstType := extractEventType(t, events[0])
	if !isStreamStartType(firstType) {
		t.Fatalf("first event type = %q, want a start event type (start, stream_start, thread.started, or response.started)", firstType)
	}

	secondType := extractEventType(t, events[1])
	if !isStreamDeltaType(secondType) {
		t.Fatalf("second event type = %q, want a delta/message event type", secondType)
	}

	lastType := extractEventType(t, events[2])
	if !isStreamEndType(lastType) {
		t.Fatalf("terminal event type = %q, want an end event type (end, stream_end, result, or response.completed)", lastType)
	}

	if status := nestedStringField(events[2], "result", "status"); status != "success" {
		t.Fatalf("terminal result.status = %q, want %q", status, "success")
	}
	if responseID := nestedStringField(events[0], "response", "id"); responseID != "resp_claude_ok" {
		t.Fatalf("start response.id = %q, want %q for deterministic snapshots", responseID, "resp_claude_ok")
	}
	if messageID := nestedStringField(events[1], "message", "id"); messageID != "msg_claude_ok" {
		t.Fatalf("assistant message.id = %q, want %q for deterministic snapshots", messageID, "msg_claude_ok")
	}
}

// TestClaudeStreamSuccessFixtureHasProvenanceAndRefreshComments verifies that
// claude_stream_success.jsonl carries the same two-line metadata convention as
// codex stream fixtures.
func TestClaudeStreamSuccessFixtureHasProvenanceAndRefreshComments(t *testing.T) {
	content := readFixtureFile(t, "claude_stream_success.jsonl")
	commentLines := extractCommentLines(content)

	if len(commentLines) < 2 {
		t.Fatalf("claude_stream_success.jsonl has %d comment lines, want at least 2 (# provenance: and # refresh:)", len(commentLines))
	}
	if !strings.HasPrefix(commentLines[0], "# provenance:") {
		t.Fatalf("first comment line = %q, want prefix %q", commentLines[0], "# provenance:")
	}
	if !strings.HasPrefix(commentLines[1], "# refresh:") {
		t.Fatalf("second comment line = %q, want prefix %q", commentLines[1], "# refresh:")
	}
}

// --- Test helpers (local to this package) ---

func readFixtureFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("failed to read fixture %q: %v", name, err)
	}
	return string(data)
}

func parseJSONLEvents(t *testing.T, content string) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(trimmed), &event); err != nil {
			t.Fatalf("invalid JSONL line %q: %v", trimmed, err)
		}
		events = append(events, event)
	}
	return events
}

func extractCommentLines(content string) []string {
	var comments []string
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			comments = append(comments, trimmed)
		}
	}
	return comments
}

func extractEventType(t *testing.T, event map[string]any) string {
	t.Helper()
	v, ok := event["type"]
	if !ok {
		t.Fatal("JSON event missing 'type' field")
	}
	s, ok := v.(string)
	if !ok {
		t.Fatal("JSON event 'type' field is not a string")
	}
	return s
}

func nestedStringField(event map[string]any, parent, child string) string {
	p, ok := event[parent]
	if !ok {
		return ""
	}
	m, ok := p.(map[string]any)
	if !ok {
		return ""
	}
	v, _ := m[child].(string)
	return v
}

func isStreamStartType(typ string) bool {
	switch typ {
	case "start", "stream_start", "thread.started", "response.started":
		return true
	}
	return false
}

func isStreamDeltaType(typ string) bool {
	switch typ {
	case "delta", "assistant", "message.delta", "response.output_text.delta":
		return true
	}
	return false
}

func isStreamEndType(typ string) bool {
	switch typ {
	case "end", "stream_end", "result", "response.completed":
		return true
	}
	return false
}

func isStreamErrorType(typ string) bool {
	switch typ {
	case "error", "stream_error", "response.error":
		return true
	}
	return false
}
