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

	t.Run("refactor and acceptance templates exclude explicit ClaudeMD blocks", func(t *testing.T) {
		templateNames := []string{
			"PROMPT_refactor.md",
			"PROMPT_acceptance_tests.md",
		}

		for _, templateName := range templateNames {
			content, err := os.ReadFile(filepath.Join(templatesDir, templateName))
			if err != nil {
				t.Fatalf("failed reading %s: %v", templateName, err)
			}

			text := string(content)
			if strings.Contains(text, "{{if .ClaudeMD}}") {
				t.Errorf("%s should not contain an explicit ClaudeMD conditional block", templateName)
			}
			if strings.Contains(text, "{{.ClaudeMD}}") {
				t.Errorf("%s should not contain explicit ClaudeMD insertion", templateName)
			}
		}
	})

	t.Run("build templates retain ClaudeMD conditional blocks", func(t *testing.T) {
		templateNames := []string{
			"PROMPT_build.md",
			"PROMPT_atdd_build.md",
			"PROMPT_tdd_build.md",
		}

		for _, templateName := range templateNames {
			content, err := os.ReadFile(filepath.Join(templatesDir, templateName))
			if err != nil {
				t.Fatalf("failed reading %s: %v", templateName, err)
			}

			text := string(content)
			if !strings.Contains(text, "{{if .ClaudeMD}}") {
				t.Errorf("%s should retain ClaudeMD conditional block", templateName)
			}
			if !strings.Contains(text, "{{.ClaudeMD}}") {
				t.Errorf("%s should retain ClaudeMD insertion", templateName)
			}
		}
	})
}
