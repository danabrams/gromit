package fixtures_test

import (
	"os"
	"path/filepath"
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
