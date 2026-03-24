package stages

import (
	"testing"
)

// TestScenario_SpecACMentionsPath_BasenameMatchesFullRelativePath verifies that
// specACMentionsPath returns true when the AC section contains the basename of a
// full relative path, even though the full path prefix differs.
func TestScenario_SpecACMentionsPath_BasenameMatchesFullRelativePath(t *testing.T) {
	specText := `# Spec 0004g

## Vision
Keep tests organized...

## Acceptance Criteria

1. Keep tests in write_contracts_test.go
2. All assertions must pass

## Scenarios
...
`
	fullPath := "internal/next/specloop/stages/write_contracts_test.go"

	result := specACMentionsPath(specText, fullPath)

	if !result {
		t.Errorf("expected specACMentionsPath to return true for basename match, got false")
	}
}
