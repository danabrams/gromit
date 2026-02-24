package fixtures_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiSpikeFindingsDocument_ContainsRequiredStructureAndRecommendations(t *testing.T) {
	path := filepath.Join("..", "..", ".gromit", "plans", "gemini-cli-spike-findings.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read findings document: %v", err)
	}

	body := string(content)
	for _, heading := range []string{
		"## 1. Prompt Delivery Modes",
		"## 2. Streaming JSON (`--output-format stream-json`)",
		"## 3. JSON Output (`--output-format json`)",
		"## 4. Token and Cost Handling",
		"## 7. Error Classification Patterns",
		"## 8. Permission Model",
		"## 9. Working Directory (CWD)",
		"## Provider-Oriented Conclusions",
		"## Limitations and Follow-up",
	} {
		if !strings.Contains(body, heading) {
			t.Fatalf("findings document must include heading %q", heading)
		}
	}

	for _, ref := range []string{
		"test/fixtures/gemini/stream-json-success.jsonl",
		"test/fixtures/gemini/json-success.json",
		"test/fixtures/gemini/commands.log",
		"test/fixtures/gemini/errors/command-missing.stderr.txt",
	} {
		if !strings.Contains(body, ref) {
			t.Fatalf("findings document must reference %q", ref)
		}
		abs := filepath.Join("..", "..", ref)
		if _, err := os.Stat(abs); err != nil {
			t.Fatalf("referenced evidence file %q must exist: %v", ref, err)
		}
	}
}
