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

	body := strings.ToLower(string(content))
	required := []string{
		"commands run",
		"observed output",
		"implementation implications",
		".gromit/plans/fixtures/gemini/stream-json-success.jsonl",
		".gromit/plans/fixtures/gemini/json-success.json",
		"recommended prompt delivery mode",
		"token",
		"cost",
		"error classification",
		"permission",
		"working directory",
		"geminiprovider",
		"version/auth caveats",
		"non-triggerable",
	}
	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("findings document must contain %q", token)
		}
	}
}
