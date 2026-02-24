package fixtures_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiSpikePlan_DocumentsCanonicalFixturePaths(t *testing.T) {
	path := filepath.Join("..", "..", ".gromit", "plans", "gemini-cli-spike.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read spike plan document: %v", err)
	}

	body := string(content)
	if strings.Contains(body, ".gromit/plans/fixtures/gemini") {
		t.Fatalf("spike plan must not reference legacy fixture path")
	}
	if !strings.Contains(body, "test/fixtures/gemini/") {
		t.Fatalf("spike plan must reference canonical fixture path")
	}
}
