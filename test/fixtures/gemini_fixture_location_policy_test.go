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

func TestFixturePolicy_DocumentsCanonicalProviderCapturePath(t *testing.T) {
	rulesPath := filepath.Join("..", "..", ".gromit", "RULES.md")
	content, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("failed to read rules file %q: %v", rulesPath, err)
	}

	body := string(content)
	if !strings.Contains(body, "Provider capture fixtures must be stored under test/fixtures/gemini/") {
		t.Fatalf("rules must define canonical provider capture fixture path under test/fixtures/gemini/")
	}
	if !strings.Contains(body, ".gromit/plans/fixtures/ is not an approved deterministic fixture path") {
		t.Fatalf("rules must explicitly reject .gromit/plans/fixtures/ for deterministic fixtures")
	}
}
