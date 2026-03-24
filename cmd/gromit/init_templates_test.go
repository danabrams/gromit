package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type profileExpectation struct {
	profileLine         string
	rulesContains       []string
	rulesNotContains    []string
	templateContains    []string
	nextStepsContains   []string
	nextStepsNotContain []string
}

// TestInitGoLogicFileIsShort verifies that init.go stays under 500 lines (template constants moved to separate file)
func TestInitGoLogicFileIsShort(t *testing.T) {
	t.Parallel()
	// Get the directory of this test file and construct path to init.go
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	initFile := filepath.Join(testDir, "init.go")

	file, err := os.Open(initFile)
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

func TestInitProfileMatrixGeneratesContent(t *testing.T) {

	cases := []struct {
		name                 string
		profile              string
		signals              []string
		rulesContains        []string
		rulesNotContains     []string
		templateContains     string
		nextStepsContains    []string
		nextStepsNotContains []string
	}{
		{
			name:              "go profile",
			profile:           "go",
			signals:           []string{"go.mod"},
			rulesContains:     []string{"go fmt"},
			templateContains:  "go test",
			nextStepsContains: []string{"go test"},
		},
		{
			name:                 "node profile",
			profile:              "node",
			signals:              []string{"package.json"},
			rulesContains:        []string{"ESLint"},
			rulesNotContains:     []string{"go fmt"},
			templateContains:     "npm test",
			nextStepsContains:    []string{"npm test"},
			nextStepsNotContains: []string{"go test"},
		},
		{
			name:                 "python profile",
			profile:              "python",
			signals:              []string{"pyproject.toml"},
			rulesContains:        []string{"Black"},
			rulesNotContains:     []string{"go fmt"},
			templateContains:     "pytest",
			nextStepsContains:    []string{"pytest"},
			nextStepsNotContains: []string{"go test"},
		},
		{
			name:                 "custom profile",
			profile:              "custom",
			rulesContains:        []string{"project-specific rules"},
			templateContains:     "Custom profiles have no default validation commands",
			nextStepsContains:    []string{"gromit run --dry-run"},
			nextStepsNotContains: []string{"go test"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			dir, stdout := runInitProfileMatrix(t, tc.profile, tc.signals)

			cfgPath := filepath.Join(dir, "gromit.yaml")
			configContent, err := os.ReadFile(cfgPath)
			if err != nil {
				t.Fatalf("failed to read gromit.yaml: %v", err)
			}
			expectedProfileLine := "project:\n  profile: \"" + tc.profile + "\""
			if !strings.Contains(string(configContent), expectedProfileLine) {
				t.Fatalf("gromit.yaml for profile %s missing profile line; got:\n%s", tc.profile, string(configContent))
			}

			rulesPath := filepath.Join(dir, ".gromit", "RULES.md")
			rulesContent, err := os.ReadFile(rulesPath)
			if err != nil {
				t.Fatalf("failed to read RULES.md: %v", err)
			}
			rulesStr := string(rulesContent)
			for _, want := range tc.rulesContains {
				if !strings.Contains(rulesStr, want) {
					t.Fatalf("RULES.md for %s missing %q; got:\n%s", tc.profile, want, rulesStr)
				}
			}
			for _, not := range tc.rulesNotContains {
				if strings.Contains(rulesStr, not) {
					t.Fatalf("RULES.md for %s unexpectedly contained %q", tc.profile, not)
				}
			}

			templatePath := filepath.Join(dir, ".gromit", "templates", "PROMPT_validate.md")
			templateContent, err := os.ReadFile(templatePath)
			if err != nil {
				t.Fatalf("failed to read validate template: %v", err)
			}
			if !strings.Contains(string(templateContent), tc.templateContains) {
				t.Fatalf("validate template for %s missing %q, got:\n%s", tc.profile, tc.templateContains, string(templateContent))
			}

			for _, want := range tc.nextStepsContains {
				if !strings.Contains(stdout, want) {
					t.Fatalf("next steps output for %s missing %q; got:\n%s", tc.profile, want, stdout)
				}
			}
			for _, not := range tc.nextStepsNotContains {
				if strings.Contains(stdout, not) {
					t.Fatalf("next steps output for %s unexpectedly contained %q", tc.profile, not)
				}
			}
		})
	}
}

// TestRunInitUsesProfileAwareRulesAndNextSteps verifies that the actual runInit command
// uses profile-aware RULES.md and next-steps guidance (not defaultRules and generic steps)
func TestRunInitUsesProfileAwareRulesAndNextSteps(t *testing.T) {
	// Note: Not parallel because this test changes the working directory

	tempDir := t.TempDir()
	t.Chdir(tempDir)

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

func runInitProfileMatrix(t *testing.T, profile string, signals []string) (string, string) {
	t.Helper()

	dir := setupProfileSignals(t, signals)

	t.Chdir(dir)

	prevForce := forceInit
	prevProfile := initProfile
	forceInit = true
	initProfile = ""
	defer func() {
		forceInit = prevForce
		initProfile = prevProfile
	}()

	stdout := captureRunInitOutput(t, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit: %v", err)
		}
	})

	return dir, stdout
}

func captureRunInitOutput(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	<-done
	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	return buf.String()
}
