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
