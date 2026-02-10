package scope

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveSpec tests that ResolveSpec returns correct labels for a spec name
func TestResolveSpec(t *testing.T) {
	tests := []struct {
		name     string
		specName string
		want     []string
	}{
		{
			name:     "simple spec name",
			specName: "auth",
			want:     []string{"spec:auth"},
		},
		{
			name:     "spec name with hyphen",
			specName: "user-auth-v2",
			want:     []string{"spec:user-auth-v2"},
		},
		{
			name:     "spec name with underscores",
			specName: "data_migration",
			want:     []string{"spec:data_migration"},
		},
		{
			name:     "empty spec name",
			specName: "",
			want:     []string{"spec:"},
		},
		{
			name:     "spec name with numbers",
			specName: "feature123",
			want:     []string{"spec:feature123"},
		},
		{
			name:     "complex spec name",
			specName: "epic-scoped-execution",
			want:     []string{"spec:epic-scoped-execution"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSpec(tt.specName)
			if len(got) != len(tt.want) {
				t.Errorf("ResolveSpec(%q) returned %d labels, want %d", tt.specName, len(got), len(tt.want))
				return
			}
			for i, label := range got {
				if label != tt.want[i] {
					t.Errorf("ResolveSpec(%q)[%d] = %q, want %q", tt.specName, i, label, tt.want[i])
				}
			}
		})
	}
}

// TestResolveSpecReturnsArray tests that ResolveSpec returns a slice, not nil
func TestResolveSpecReturnsArray(t *testing.T) {
	tests := []struct {
		name     string
		specName string
	}{
		{"empty string", ""},
		{"simple name", "auth"},
		{"complex name", "user-profile-v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSpec(tt.specName)
			if got == nil {
				t.Errorf("ResolveSpec(%q) returned nil, want non-nil slice", tt.specName)
			}
		})
	}
}

// TestResolveSpecLabelFormat tests that labels follow the spec:name format
func TestResolveSpecLabelFormat(t *testing.T) {
	tests := []struct {
		name     string
		specName string
	}{
		{"simple", "auth"},
		{"with-hyphens", "user-auth"},
		{"with_underscores", "data_migration"},
		{"mixed", "feature-v2_beta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSpec(tt.specName)
			if len(got) == 0 {
				t.Fatalf("ResolveSpec(%q) returned empty slice", tt.specName)
			}
			label := got[0]
			expectedPrefix := "spec:"
			if !strings.HasPrefix(label, expectedPrefix) {
				t.Errorf("ResolveSpec(%q) = %q, want label with prefix %q", tt.specName, label, expectedPrefix)
			}
			expectedSuffix := tt.specName
			if !strings.HasSuffix(label, expectedSuffix) {
				t.Errorf("ResolveSpec(%q) = %q, want label with suffix %q", tt.specName, label, expectedSuffix)
			}
			expectedFull := "spec:" + tt.specName
			if label != expectedFull {
				t.Errorf("ResolveSpec(%q) = %q, want %q", tt.specName, label, expectedFull)
			}
		})
	}
}

// TestValidateFlags_EpicAndSpecOnly tests ValidateFlags with two parameters (backward compatibility)
func TestValidateFlags_EpicAndSpecOnly(t *testing.T) {
	tests := []struct {
		name    string
		epic    string
		spec    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "both flags set with non-empty values",
			epic:    "gromit-xyz",
			spec:    "init-wizard",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "epic set, spec empty",
			epic:    "gromit-xyz",
			spec:    "",
			wantErr: false,
		},
		{
			name:    "spec set, epic empty",
			epic:    "",
			spec:    "init-wizard",
			wantErr: false,
		},
		{
			name:    "both flags empty",
			epic:    "",
			spec:    "",
			wantErr: false,
		},
		{
			name:    "both flags with same value",
			epic:    "same-value",
			spec:    "same-value",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "epic with whitespace, spec empty",
			epic:    "  gromit-xyz  ",
			spec:    "",
			wantErr: false,
		},
		{
			name:    "spec with whitespace, epic empty",
			epic:    "",
			spec:    "  init-wizard  ",
			wantErr: false,
		},
		{
			name:    "both flags with whitespace",
			epic:    "  epic  ",
			spec:    "  spec  ",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.epic, tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFlags(%q, %q) error = %v, wantErr %v", tt.epic, tt.spec, err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateFlags(%q, %q) error = %q, want error containing %q", tt.epic, tt.spec, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestValidateFlagsThreeWay tests ValidateFlags with all three parameters including since
func TestValidateFlagsThreeWay(t *testing.T) {
	tests := []struct {
		name    string
		epic    string
		spec    string
		since   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "epic and since both set",
			epic:    "gromit-xyz",
			spec:    "",
			since:   "abc123",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "spec and since both set",
			epic:    "",
			spec:    "init-wizard",
			since:   "abc123",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "epic and spec both set (no since)",
			epic:    "gromit-xyz",
			spec:    "init-wizard",
			since:   "",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "all three flags set",
			epic:    "gromit-xyz",
			spec:    "init-wizard",
			since:   "abc123",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name:    "only epic set",
			epic:    "gromit-xyz",
			spec:    "",
			since:   "",
			wantErr: false,
		},
		{
			name:    "only spec set",
			epic:    "",
			spec:    "init-wizard",
			since:   "",
			wantErr: false,
		},
		{
			name:    "only since set",
			epic:    "",
			spec:    "",
			since:   "abc123",
			wantErr: false,
		},
		{
			name:    "no flags set",
			epic:    "",
			spec:    "",
			since:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.epic, tt.spec, tt.since)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFlags(%q, %q, %q) error = %v, wantErr %v", tt.epic, tt.spec, tt.since, err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateFlags(%q, %q, %q) error = %q, want error containing %q", tt.epic, tt.spec, tt.since, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestValidateFlagsErrorMessage tests that the error message is clear and helpful
func TestValidateFlagsErrorMessage(t *testing.T) {
	err := ValidateFlags("gromit-xyz", "init-wizard")
	if err == nil {
		t.Fatal("ValidateFlags with both flags should return error")
	}

	errMsg := err.Error()
	// Error message should mention both flags
	if !strings.Contains(strings.ToLower(errMsg), "epic") && !strings.Contains(strings.ToLower(errMsg), "spec") {
		t.Errorf("error message should mention 'epic' or 'spec', got: %q", errMsg)
	}
	// Error message should indicate they can't be used together
	mustContainOne := []string{"mutually exclusive", "cannot", "both", "together"}
	containsOne := false
	for _, phrase := range mustContainOne {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(phrase)) {
			containsOne = true
			break
		}
	}
	if !containsOne {
		t.Errorf("error message should indicate flags can't be used together, got: %q", errMsg)
	}
}

// TestValidateFlagsWithTrimmableWhitespace tests that validation considers trimmed values
func TestValidateFlagsWithTrimmableWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		epic    string
		spec    string
		wantErr bool
	}{
		{
			name:    "epic with spaces trims to empty",
			epic:    "   ",
			spec:    "",
			wantErr: false,
		},
		{
			name:    "spec with spaces trims to empty",
			epic:    "",
			spec:    "   ",
			wantErr: false,
		},
		{
			name:    "both with spaces trim to empty",
			epic:    "   ",
			spec:    "   ",
			wantErr: false,
		},
		{
			name:    "epic with leading/trailing spaces and spec set",
			epic:    "  epic-123  ",
			spec:    "spec-456",
			wantErr: true,
		},
		{
			name:    "spec with leading/trailing spaces and epic set",
			epic:    "epic-123",
			spec:    "  spec-456  ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.epic, tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFlags(%q, %q) error = %v, wantErr %v", tt.epic, tt.spec, err, tt.wantErr)
			}
		})
	}
}

// TestResolveSpecWithSpecialCharacters tests spec names with various characters
func TestResolveSpecWithSpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		specName string
		want     string
	}{
		{"with dot", "v2.5", "spec:v2.5"},
		{"with multiple hyphens", "user---auth", "spec:user---auth"},
		{"with mixed separators", "epic_scoped-execution.v2", "spec:epic_scoped-execution.v2"},
		{"numeric only", "123", "spec:123"},
		{"single character", "a", "spec:a"},
		{"uppercase", "AUTH", "spec:AUTH"},
		{"mixed case", "UserAuth", "spec:UserAuth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSpec(tt.specName)
			if len(got) == 0 {
				t.Fatalf("ResolveSpec(%q) returned empty slice", tt.specName)
			}
			if got[0] != tt.want {
				t.Errorf("ResolveSpec(%q) = %q, want %q", tt.specName, got[0], tt.want)
			}
		})
	}
}

// TestResolveSpecReturnsSingleLabel tests that ResolveSpec returns exactly one label
func TestResolveSpecReturnsSingleLabel(t *testing.T) {
	tests := []string{
		"simple",
		"with-hyphens",
		"with_underscores",
		"",
		"123",
		"MixedCase",
	}

	for _, specName := range tests {
		t.Run(specName, func(t *testing.T) {
			got := ResolveSpec(specName)
			if len(got) != 1 {
				t.Errorf("ResolveSpec(%q) returned %d labels, want exactly 1", specName, len(got))
			}
		})
	}
}

// TestValidateFlagsThreeWay_TrimsWhitespace tests that three-way validation
// considers trimmed values when checking mutual exclusivity
func TestValidateFlagsThreeWay_TrimsWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		epic    string
		spec    string
		since   string
		wantErr bool
	}{
		{
			name:    "whitespace-only values treated as empty",
			epic:    "   ",
			spec:    "   ",
			since:   "   ",
			wantErr: false,
		},
		{
			name:    "epic with value, spec with whitespace, since empty",
			epic:    "gromit-xyz",
			spec:    "   ",
			since:   "",
			wantErr: false,
		},
		{
			name:    "epic with whitespace, spec with value, since with value",
			epic:    "   ",
			spec:    "init-wizard",
			since:   "abc123",
			wantErr: true,
		},
		{
			name:    "leading/trailing spaces in since",
			epic:    "",
			spec:    "",
			since:   "  abc123  ",
			wantErr: false,
		},
		{
			name:    "two params with spaces - should error",
			epic:    "  gromit-xyz  ",
			spec:    "  init-wizard  ",
			since:   "",
			wantErr: true,
		},
		{
			name:    "whitespace-only since with real epic",
			epic:    "gromit-xyz",
			spec:    "",
			since:   "   ",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.epic, tt.spec, tt.since)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFlags(%q, %q, %q) error = %v, wantErr %v",
					tt.epic, tt.spec, tt.since, err, tt.wantErr)
			}
		})
	}
}

// TestValidateFlagsThreeWay_MentionsFlags verifies that error messages mention
// which flags are conflicting
func TestValidateFlagsThreeWay_MentionsFlags(t *testing.T) {
	tests := []struct {
		name          string
		epic          string
		spec          string
		since         string
		shouldMention []string
	}{
		{
			name:          "epic and spec conflict",
			epic:          "gromit-xyz",
			spec:          "init-wizard",
			since:         "",
			shouldMention: []string{"epic", "spec"},
		},
		{
			name:          "epic and since conflict",
			epic:          "gromit-xyz",
			spec:          "",
			since:         "abc123",
			shouldMention: []string{"epic", "since"},
		},
		{
			name:          "spec and since conflict",
			epic:          "",
			spec:          "init-wizard",
			since:         "abc123",
			shouldMention: []string{"spec", "since"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.epic, tt.spec, tt.since)
			if err == nil {
				t.Fatalf("ValidateFlags should return error for %s", tt.name)
			}

			errMsg := strings.ToLower(err.Error())
			for _, flag := range tt.shouldMention {
				if !strings.Contains(errMsg, flag) {
					t.Errorf("error message should mention %q flag, got: %q", flag, err.Error())
				}
			}
		})
	}
}

// TestResolveEpic_NoSpecsDir tests ResolveEpic when specs directory doesn't exist
func TestResolveEpic_NoSpecsDir(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentDir := filepath.Join(tempDir, "nonexistent")

	labels, err := ResolveEpic("gromit-xyz", nonExistentDir)
	if err == nil {
		t.Fatal("ResolveEpic with nonexistent directory should return error")
	}
	if labels != nil {
		t.Errorf("ResolveEpic with nonexistent directory should return nil labels, got %v", labels)
	}
}

// TestResolveEpic_EmptySpecsDir tests ResolveEpic when specs directory is empty
func TestResolveEpic_EmptySpecsDir(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic with empty directory returned error: %v", err)
	}
	if len(labels) != 0 {
		t.Errorf("ResolveEpic with empty directory should return empty slice, got %v", labels)
	}
}

// TestResolveEpic_NoMatchingSpecs tests ResolveEpic when no specs match the epic
func TestResolveEpic_NoMatchingSpecs(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec without epic field
	specPath := filepath.Join(specsDir, "auth.md")
	specContent := `---
id: auth
created: 2026-02-08
---

# Authentication

Some spec content here.
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 0 {
		t.Errorf("ResolveEpic with no matching specs should return empty slice, got %v", labels)
	}
}

// TestResolveEpic_SingleMatchingSpec tests ResolveEpic with one matching spec
func TestResolveEpic_SingleMatchingSpec(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec with matching epic field
	specPath := filepath.Join(specsDir, "user-auth.md")
	specContent := `---
id: user-auth
epic: gromit-xyz
created: 2026-02-08
---

# User Authentication

Some spec content here.
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("ResolveEpic should return 1 label, got %d", len(labels))
	}
	expectedLabel := "spec:user-auth"
	if labels[0] != expectedLabel {
		t.Errorf("ResolveEpic returned label %q, want %q", labels[0], expectedLabel)
	}
}

// TestResolveEpic_MultipleMatchingSpecs tests ResolveEpic with multiple matching specs
func TestResolveEpic_MultipleMatchingSpecs(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create multiple specs with matching epic field
	specs := []struct {
		filename string
		id       string
	}{
		{"user-auth.md", "user-auth"},
		{"user-profile.md", "user-profile"},
		{"user-settings.md", "user-settings"},
	}

	for _, spec := range specs {
		specPath := filepath.Join(specsDir, spec.filename)
		specContent := fmt.Sprintf(`---
id: %s
epic: gromit-xyz
created: 2026-02-08
---

# Spec

Content.
`, spec.id)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file %s: %v", spec.filename, err)
		}
	}

	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 3 {
		t.Fatalf("ResolveEpic should return 3 labels, got %d", len(labels))
	}

	// Verify all expected labels are present
	expectedLabels := map[string]bool{
		"spec:user-auth":     false,
		"spec:user-profile":  false,
		"spec:user-settings": false,
	}
	for _, label := range labels {
		if _, exists := expectedLabels[label]; !exists {
			t.Errorf("Unexpected label %q", label)
		}
		expectedLabels[label] = true
	}
	for label, found := range expectedLabels {
		if !found {
			t.Errorf("Missing expected label %q", label)
		}
	}
}

// TestResolveEpic_MixedSpecs tests ResolveEpic with specs belonging to different epics
func TestResolveEpic_MixedSpecs(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create specs with different epic fields
	specs := []struct {
		filename string
		id       string
		epic     string
	}{
		{"auth.md", "auth", "gromit-xyz"},
		{"profile.md", "profile", "gromit-xyz"},
		{"settings.md", "settings", "gromit-abc"},
		{"dashboard.md", "dashboard", "gromit-def"},
	}

	for _, spec := range specs {
		specPath := filepath.Join(specsDir, spec.filename)
		specContent := fmt.Sprintf(`---
id: %s
epic: %s
created: 2026-02-08
---

# Spec

Content.
`, spec.id, spec.epic)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file %s: %v", spec.filename, err)
		}
	}

	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("ResolveEpic should return 2 labels for gromit-xyz, got %d", len(labels))
	}

	// Verify only gromit-xyz specs are returned
	for _, label := range labels {
		if label != "spec:auth" && label != "spec:profile" {
			t.Errorf("Unexpected label %q for epic gromit-xyz", label)
		}
	}
}

// TestResolveEpic_IgnoresNonMarkdownFiles tests that ResolveEpic ignores non-.md files
func TestResolveEpic_IgnoresNonMarkdownFiles(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create .md file with matching epic
	mdPath := filepath.Join(specsDir, "auth.md")
	mdContent := `---
id: auth
epic: gromit-xyz
created: 2026-02-08
---

# Auth
`
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		t.Fatalf("Failed to write .md file: %v", err)
	}

	// Create .txt file with matching epic (should be ignored)
	txtPath := filepath.Join(specsDir, "readme.txt")
	txtContent := `---
id: readme
epic: gromit-xyz
created: 2026-02-08
---

# Readme
`
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		t.Fatalf("Failed to write .txt file: %v", err)
	}

	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("ResolveEpic should return 1 label (only .md file), got %d", len(labels))
	}
	if labels[0] != "spec:auth" {
		t.Errorf("ResolveEpic returned label %q, want spec:auth", labels[0])
	}
}

// TestResolveEpic_MissingIdField tests ResolveEpic when spec has epic but no id field
func TestResolveEpic_MissingIdField(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec with epic but no id field
	specPath := filepath.Join(specsDir, "broken.md")
	specContent := `---
epic: gromit-xyz
created: 2026-02-08
---

# Broken Spec

Missing id field.
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Should skip specs without id field
	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 0 {
		t.Errorf("ResolveEpic should skip specs without id field, got labels: %v", labels)
	}
}

// TestResolveEpic_InvalidFrontmatter tests ResolveEpic with malformed frontmatter
func TestResolveEpic_InvalidFrontmatter(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec with invalid YAML
	specPath := filepath.Join(specsDir, "broken.md")
	specContent := `---
id: broken
epic: gromit-xyz
invalid yaml: [unclosed
---

# Broken Spec
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// ResolveEpic should handle or skip invalid specs
	_, err := ResolveEpic("gromit-xyz", specsDir)
	// Could either skip the file and return empty, or return an error
	// The implementation will decide, but it shouldn't panic
	if err != nil {
		// Error is acceptable for invalid frontmatter
		t.Logf("ResolveEpic returned error for invalid frontmatter (acceptable): %v", err)
	}
}

// TestResolveEpic_SpecsWithNoEpicField tests specs without epic field are ignored
func TestResolveEpic_SpecsWithNoEpicField(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec with matching epic
	withEpic := filepath.Join(specsDir, "with-epic.md")
	withEpicContent := `---
id: with-epic
epic: gromit-xyz
created: 2026-02-08
---

# With Epic
`
	if err := os.WriteFile(withEpic, []byte(withEpicContent), 0644); err != nil {
		t.Fatalf("Failed to write spec with epic: %v", err)
	}

	// Create spec without epic field
	withoutEpic := filepath.Join(specsDir, "without-epic.md")
	withoutEpicContent := `---
id: without-epic
created: 2026-02-08
---

# Without Epic
`
	if err := os.WriteFile(withoutEpic, []byte(withoutEpicContent), 0644); err != nil {
		t.Fatalf("Failed to write spec without epic: %v", err)
	}

	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("ResolveEpic should return 1 label, got %d", len(labels))
	}
	if labels[0] != "spec:with-epic" {
		t.Errorf("ResolveEpic returned label %q, want spec:with-epic", labels[0])
	}
}

// TestResolveEpic_EpicFieldTypeString tests that epic field must be a string
func TestResolveEpic_EpicFieldTypeString(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec with epic as string
	validPath := filepath.Join(specsDir, "valid.md")
	validContent := `---
id: valid
epic: gromit-xyz
created: 2026-02-08
---

# Valid
`
	if err := os.WriteFile(validPath, []byte(validContent), 0644); err != nil {
		t.Fatalf("Failed to write valid spec: %v", err)
	}

	// Create spec with epic as non-string (should be ignored or error)
	invalidPath := filepath.Join(specsDir, "invalid.md")
	invalidContent := `---
id: invalid
epic: 123
created: 2026-02-08
---

# Invalid
`
	if err := os.WriteFile(invalidPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid spec: %v", err)
	}

	// Should only return label for valid spec
	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("ResolveEpic should return 1 label (skip non-string epic), got %d", len(labels))
	}
	if labels[0] != "spec:valid" {
		t.Errorf("ResolveEpic returned label %q, want spec:valid", labels[0])
	}
}

// TestResolveEpic_EmptyEpicID tests behavior when epicID is empty string
func TestResolveEpic_EmptyEpicID(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec with empty epic field
	specPath := filepath.Join(specsDir, "empty-epic.md")
	specContent := `---
id: empty-epic
epic: ""
created: 2026-02-08
---

# Empty Epic
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	labels, err := ResolveEpic("", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic with empty epicID returned error: %v", err)
	}
	// Should match specs with empty epic field
	if len(labels) != 1 {
		t.Errorf("ResolveEpic with empty epicID should match empty epic fields, got %d labels", len(labels))
	}
}

// TestResolveEpic_LabelFormat tests that returned labels follow spec:id format
func TestResolveEpic_LabelFormat(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	specIDs := []string{"auth", "user-profile", "settings_v2", "feature123"}
	for _, id := range specIDs {
		specPath := filepath.Join(specsDir, id+".md")
		specContent := fmt.Sprintf(`---
id: %s
epic: gromit-xyz
created: 2026-02-08
---

# Spec
`, id)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	labels, err := ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}

	for _, label := range labels {
		if !strings.HasPrefix(label, "spec:") {
			t.Errorf("Label %q should have prefix 'spec:'", label)
		}
		// Extract ID part and verify it's one of our spec IDs
		labelID := strings.TrimPrefix(label, "spec:")
		found := false
		for _, id := range specIDs {
			if labelID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Label %q has unexpected ID %q", label, labelID)
		}
	}
}
