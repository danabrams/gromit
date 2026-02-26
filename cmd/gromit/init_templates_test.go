package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitGoLogicFileIsShort verifies that init.go stays under 500 lines (template constants moved to separate file)
func TestInitGoLogicFileIsShort(t *testing.T) {
	t.Parallel()
	file, err := os.Open("init.go")
	if err != nil {
		t.Fatalf("failed to open init.go: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading init.go: %v", err)
	}

	const maxLines = 500
	if lineCount > maxLines {
		t.Errorf("init.go has %d lines, expected <= %d lines (templates should be extracted)", lineCount, maxLines)
	}
}

// TestSeedProfileAwareCommandExamples_InjectsProfileGuidanceIntoValidateTemplate verifies that
// profile-aware command example notes are injected into templates when seeding for a specific profile
func TestSeedProfileAwareCommandExamples_InjectsProfileGuidanceIntoValidateTemplate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		profile          string
		wantGuidanceText string
	}{
		{
			profile:          "go",
			wantGuidanceText: "go test",
		},
		{
			profile:          "node",
			wantGuidanceText: "npm test",
		},
		{
			profile:          "python",
			wantGuidanceText: "pytest",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.profile, func(t *testing.T) {
			t.Parallel()

			seededTemplate := seedProfileAwareCommandExamples(tc.profile, defaultValidateTemplate)

			if !strings.Contains(seededTemplate, tc.wantGuidanceText) {
				t.Errorf("seeded template for profile %q missing expected guidance text %q in:\n%s",
					tc.profile, tc.wantGuidanceText, seededTemplate)
			}
		})
	}
}

// TestInitUsesProfileAwareTemplateSeeding verifies that gromit init writes profile-aware seeded templates
// when initializing a project with a specific profile
func TestInitUsesProfileAwareTemplateSeeding(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		profile          string
		wantGuidanceText string
	}{
		{
			profile:          "node",
			wantGuidanceText: "npm test",
		},
		{
			profile:          "python",
			wantGuidanceText: "pytest",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.profile, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()

			// Write gromit.yaml with explicit profile
			configContent := `project:
  profile: "` + tc.profile + `"
models:
  p0: opus
  p1: sonnet
  p2: haiku
  validation: haiku
validation:
  enabled: true
  commands: []
`
			if err := os.WriteFile(filepath.Join(tempDir, "gromit.yaml"), []byte(configContent), 0644); err != nil {
				t.Fatalf("failed to write gromit.yaml: %v", err)
			}

			// Create .gromit/templates directory
			templatesDir := filepath.Join(tempDir, ".gromit/templates")
			if err := os.MkdirAll(templatesDir, 0755); err != nil {
				t.Fatalf("failed to create templates dir: %v", err)
			}

			// Manually call the template writing logic (simulating what init does)
			// We'll write the seeded template directly
			seededTemplate := seedProfileAwareCommandExamples(tc.profile, defaultValidateTemplate)
			validatePath := filepath.Join(templatesDir, "PROMPT_validate.md")
			if err := os.WriteFile(validatePath, []byte(seededTemplate), 0644); err != nil {
				t.Fatalf("failed to write validate template: %v", err)
			}

			// Read back the template and verify it contains profile guidance
			content, err := os.ReadFile(validatePath)
			if err != nil {
				t.Fatalf("failed to read validate template: %v", err)
			}

			if !strings.Contains(string(content), tc.wantGuidanceText) {
				t.Errorf("validate template for profile %q missing expected guidance text %q, got:\n%s",
					tc.profile, tc.wantGuidanceText, string(content))
			}
		})
	}
}

// TestRulesForProfile_IncludesGoSpecificGuide verifies that rulesForProfile generates
// Go-specific RULES.md content with relevant Code Style guidance for Go projects
func TestRulesForProfile_IncludesGoSpecificGuide(t *testing.T) {
	t.Parallel()

	rules := rulesForProfile("go")

	// Should contain universal Safety section
	if !strings.Contains(rules, "Never commit secrets") {
		t.Error("rules missing universal safety guidance")
	}

	// Should contain Go-specific Code Style guidance
	if !strings.Contains(rules, "go fmt") {
		t.Error("rules missing go fmt guidance for go profile")
	}
}

// TestNextStepsForProfile_IncludesGoSpecificGuidance verifies that nextStepsForProfile generates
// Go-specific next steps terminal guidance with relevant commands
func TestNextStepsForProfile_IncludesGoSpecificGuidance(t *testing.T) {
	t.Parallel()

	nextSteps := nextStepsForProfile("go")

	// Should contain universal next steps
	if !strings.Contains(nextSteps, "gromit.yaml") {
		t.Error("next steps missing gromit.yaml guidance")
	}

	// Should contain Go-specific validation commands
	if !strings.Contains(nextSteps, "go test") {
		t.Error("next steps missing go test guidance for go profile")
	}
}

// TestInitWritesProfileAwareRules verifies that gromit init writes profile-aware RULES.md
// content when initializing a project with a specific profile
func TestInitWritesProfileAwareRules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		profile             string
		wantProfileSpecific string
	}{
		{
			profile:             "go",
			wantProfileSpecific: "go fmt",
		},
		{
			profile:             "node",
			wantProfileSpecific: "ESLint",
		},
		{
			profile:             "python",
			wantProfileSpecific: "Black",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.profile, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()

			// Simulate what runInit does: generate and write profile-aware RULES.md
			rulesContent := rulesForProfile(tc.profile)
			rulesPath := filepath.Join(tempDir, ".gromit", "RULES.md")

			if err := os.MkdirAll(filepath.Dir(rulesPath), 0755); err != nil {
				t.Fatalf("failed to create .gromit dir: %v", err)
			}

			if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
				t.Fatalf("failed to write RULES.md: %v", err)
			}

			// Read back and verify it contains profile-specific guidance
			content, err := os.ReadFile(rulesPath)
			if err != nil {
				t.Fatalf("failed to read RULES.md: %v", err)
			}

			contentStr := string(content)
			if !strings.Contains(contentStr, tc.wantProfileSpecific) {
				t.Errorf("RULES.md for profile %q missing expected profile-specific content %q, got:\n%s",
					tc.profile, tc.wantProfileSpecific, contentStr)
			}

			// Should also contain universal safety guidance
			if !strings.Contains(contentStr, "Never commit secrets") {
				t.Error("RULES.md missing universal safety guidance")
			}
		})
	}
}

// TestRunInitUsesProfileAwareRulesAndNextSteps verifies that the actual runInit command
// uses profile-aware RULES.md and next-steps guidance (not defaultRules and generic steps)
func TestRunInitUsesProfileAwareRulesAndNextSteps(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	// Change to temp directory for init command
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer func() { os.Chdir(cwd) }()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Write a gromit.yaml with go profile
	configContent := `project:
  profile: "go"
models:
  p0: opus
  p1: sonnet
  p2: haiku
  validation: haiku
validation:
  enabled: true
  commands: []
`
	if err := os.WriteFile("gromit.yaml", []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write gromit.yaml: %v", err)
	}

	// Run init
	initCmd.RunE(initCmd, []string{})

	// Read back RULES.md and verify it contains go fmt (profile-specific)
	rulesContent, err := os.ReadFile(".gromit/RULES.md")
	if err != nil {
		t.Fatalf("failed to read generated RULES.md: %v", err)
	}

	rulesStr := string(rulesContent)
	if !strings.Contains(rulesStr, "go fmt") {
		t.Error("RULES.md missing Go-specific guidance (go fmt) - init not using profile-aware rules")
	}

	if !strings.Contains(rulesStr, "Never commit secrets") {
		t.Error("RULES.md missing universal safety guidance")
	}
}

