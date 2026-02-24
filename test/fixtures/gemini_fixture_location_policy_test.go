package fixtures_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiCaptureFixtures_UseCanonicalTestFixturesLocation(t *testing.T) {
	legacyDir := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini")
	canonicalDir := filepath.Join("..", "..", "test", "fixtures", "gemini")

	if _, err := os.Stat(canonicalDir); err != nil {
		t.Fatalf("canonical fixture directory must exist at %q: %v", canonicalDir, err)
	}

	if _, err := os.Stat(legacyDir); err == nil {
		t.Fatalf("legacy fixture directory %q must not exist; use %q", legacyDir, canonicalDir)
	}
}

func TestGeminiFixtureTests_DoNotHardcodeLegacyPathJoin(t *testing.T) {
	sourcePath := filepath.Join("..", "..", "test", "fixtures", "gemini_preflight_fixture_test.go")
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("failed to read fixture test source %q: %v", sourcePath, err)
	}

	legacyJoin := "\".gromit\", \"plans\", \"fixtures\", \"gemini\""
	if strings.Contains(string(content), legacyJoin) {
		t.Fatalf("fixture test source must use geminiFixturePath helper instead of legacy join pattern %s", legacyJoin)
	}
}
