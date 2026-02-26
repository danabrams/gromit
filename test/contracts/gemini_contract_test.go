//go:build contract

package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeminiContractFixtures_ExistWithProvenance verifies that canonical Gemini fixture files exist.
func TestGeminiContractFixtures_ExistWithProvenance(t *testing.T) {
	required := []string{
		"gemini_success.txt",
		"gemini_stream_success.jsonl",
		"gemini_stream_failure.jsonl",
	}

	for _, name := range required {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(fixturesDir, name)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("expected canonical fixture %q to exist: %v", name, err)
			}

			if strings.TrimSpace(string(content)) == "" {
				t.Fatalf("expected fixture %q to contain realistic payload content", name)
			}

			lower := strings.ToLower(string(content))
			if !strings.Contains(lower, "provenance") {
				t.Fatalf("expected fixture %q to include a provenance comment for refresh workflow", name)
			}
		})
	}
}

// TestGeminiContractFixtures_StreamFixturesUseLifecycleAndErrorShapes verifies that
// gemini stream fixtures follow the init/message/result lifecycle pattern.
func TestGeminiContractFixtures_StreamFixturesUseLifecycleAndErrorShapes(t *testing.T) {
	tests := []struct {
		name                      string
		fixtureName               string
		requireLifecycleStart     bool
		requireTerminalErrorEvent bool
	}{
		{
			name:                  "gemini stream success has init message result lifecycle",
			fixtureName:           "gemini_stream_success.jsonl",
			requireLifecycleStart: true,
		},
		{
			name:                      "gemini stream failure ends with explicit error event",
			fixtureName:               "gemini_stream_failure.jsonl",
			requireLifecycleStart:     true,
			requireTerminalErrorEvent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(fixturesDir, tt.fixtureName)
			contentBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture %q: %v", tt.fixtureName, err)
			}

			commentLines, events := parseJSONLFixture(t, string(contentBytes))
			if len(commentLines) < 2 {
				t.Fatalf("fixture %q must start with two comment lines (# provenance: and # refresh:)", tt.fixtureName)
			}
			if !strings.HasPrefix(commentLines[0], "# provenance:") {
				t.Fatalf("fixture %q first comment must start with '# provenance:'", tt.fixtureName)
			}
			if !strings.HasPrefix(commentLines[1], "# refresh:") {
				t.Fatalf("fixture %q second comment must start with '# refresh:'", tt.fixtureName)
			}

			if len(events) < 3 {
				t.Fatalf("fixture %q must contain at least 3 JSON events (init/start, message(s), result/error)", tt.fixtureName)
			}

			if tt.requireLifecycleStart {
				firstType := eventType(events[0])
				if firstType != "init" {
					t.Fatalf("fixture %q first event type = %q, want 'init'", tt.fixtureName, firstType)
				}

				hasMessage := false
				for _, event := range events[1 : len(events)-1] {
					if eventType(event) == "message" {
						hasMessage = true
						break
					}
				}
				if !hasMessage {
					t.Fatalf("fixture %q must include at least one message event between init and terminal events", tt.fixtureName)
				}
			}

			last := events[len(events)-1]
			lastType := eventType(last)
			if lastType != "result" {
				t.Fatalf("fixture %q terminal event type = %q, want 'result'", tt.fixtureName, lastType)
			}

			if tt.requireTerminalErrorEvent {
				if !hasResultError(last) {
					t.Fatalf("fixture %q terminal result event must include error status", tt.fixtureName)
				}
			} else {
				if hasResultError(last) {
					t.Fatalf("fixture %q success terminal result should not have error status", tt.fixtureName)
				}
			}
		})
	}
}

// hasResultError checks if a result event has an error field.
func hasResultError(event map[string]any) bool {
	if _, ok := event["error"]; ok {
		return true
	}
	return false
}
