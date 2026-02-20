package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateClaudeMDScope(t *testing.T) {
	templatesDir := filepath.Join("..", "..", ".gromit", "templates")
	if _, err := os.Stat(filepath.Join(templatesDir, "PROMPT_build.md")); os.IsNotExist(err) {
		t.Skipf("skipping: real templates not found at %s", templatesDir)
	}

	assertTemplateClaudeMDMarkers := func(t *testing.T, templateNames []string, wantConditional, wantInsertion bool) {
		t.Helper()

		for _, templateName := range templateNames {
			content, err := os.ReadFile(filepath.Join(templatesDir, templateName))
			if err != nil {
				t.Fatalf("failed reading %s: %v", templateName, err)
			}

			text := string(content)
			hasConditional := strings.Contains(text, "{{if .ClaudeMD}}")
			hasInsertion := strings.Contains(text, "{{.ClaudeMD}}")

			if hasConditional != wantConditional {
				if wantConditional {
					t.Errorf("%s should retain ClaudeMD conditional block", templateName)
				} else {
					t.Errorf("%s should not contain an explicit ClaudeMD conditional block", templateName)
				}
			}
			if hasInsertion != wantInsertion {
				if wantInsertion {
					t.Errorf("%s should retain ClaudeMD insertion", templateName)
				} else {
					t.Errorf("%s should not contain explicit ClaudeMD insertion", templateName)
				}
			}
		}
	}

	t.Run("refactor and acceptance templates exclude explicit ClaudeMD blocks", func(t *testing.T) {
		assertTemplateClaudeMDMarkers(t, []string{
			"PROMPT_refactor.md",
			"PROMPT_acceptance_tests.md",
		}, false, false)
	})

	t.Run("build templates retain ClaudeMD conditional blocks", func(t *testing.T) {
		assertTemplateClaudeMDMarkers(t, []string{
			"PROMPT_build.md",
			"PROMPT_atdd_build.md",
			"PROMPT_tdd_build.md",
		}, true, true)
	})
}
