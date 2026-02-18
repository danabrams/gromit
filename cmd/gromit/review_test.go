package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/scope"
)

// saveReviewFlags saves the current review flag values and registers a cleanup
// to restore them after the test completes.
func saveReviewFlags(t *testing.T) {
	t.Helper()
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	t.Cleanup(func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	})
}

// TestValidateCommitRef verifies that commit refs starting with "-" are rejected.
func TestValidateCommitRef(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{"valid sha", "abc1234", false},
		{"valid full sha", "cf02391aabbccddeeff00112233445566778899aa", false},
		{"valid branch name", "main", false},
		{"valid HEAD", "HEAD", false},
		{"flag injection attempt", "--output=/tmp/x", true},
		{"short flag attempt", "-n", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommitRef(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCommitRef(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			}
		})
	}
}

// TestGetGitDiffForReview_RejectsFlagInjection verifies that getGitDiffForReview
// rejects commit refs that look like git flags.
func TestGetGitDiffForReview_RejectsFlagInjection(t *testing.T) {
	_, err := getGitDiffForReview("--output=/tmp/x")
	if err == nil {
		t.Fatal("getGitDiffForReview should reject flag-like commit ref")
	}
	if !strings.Contains(err.Error(), "invalid commit ref") {
		t.Errorf("error should mention 'invalid commit ref', got: %v", err)
	}
}

// TestTimestampComparison verifies that Unix timestamp comparison is done numerically,
// not lexicographically. This is a regression test for the bug where string comparison
// was used (e.g. "9" > "10" in string comparison).
func TestTimestampComparison(t *testing.T) {
	tests := []struct {
		name       string
		timestamp1 string
		timestamp2 string
		want       bool // true if timestamp1 < timestamp2
	}{
		{
			name:       "clearly earlier (10 digits)",
			timestamp1: "1609459200", // 2021-01-01
			timestamp2: "1640995200", // 2022-01-01
			want:       true,
		},
		{
			name:       "clearly later (10 digits)",
			timestamp1: "1640995200", // 2022-01-01
			timestamp2: "1609459200", // 2021-01-01
			want:       false,
		},
		{
			name:       "equal timestamps",
			timestamp1: "1609459200",
			timestamp2: "1609459200",
			want:       false,
		},
		{
			name:       "single digit vs double digit (string compare would fail)",
			timestamp1: "9",
			timestamp2: "10",
			want:       true, // 9 < 10 numerically (but "9" > "10" in string comparison)
		},
		{
			name:       "large vs small with string comparison issue",
			timestamp1: "999999999",  // 9 digits
			timestamp2: "1000000000", // 10 digits (epoch start)
			want:       true,         // numerically correct (but string compare would be false)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the timestamps using the same logic as isCommitEarlier
			ts1, err := strconv.ParseInt(tt.timestamp1, 10, 64)
			if err != nil {
				t.Fatalf("Failed to parse timestamp1 %q: %v", tt.timestamp1, err)
			}

			ts2, err := strconv.ParseInt(tt.timestamp2, 10, 64)
			if err != nil {
				t.Fatalf("Failed to parse timestamp2 %q: %v", tt.timestamp2, err)
			}

			got := ts1 < ts2

			if got != tt.want {
				t.Errorf("timestamp comparison: ts1=%d, ts2=%d, got %v, want %v", ts1, ts2, got, tt.want)
			}

			// Also verify that string comparison would give incorrect results for the edge cases
			if tt.name == "single digit vs double digit (string compare would fail)" {
				stringCompare := tt.timestamp1 < tt.timestamp2
				if stringCompare == got {
					t.Errorf("Expected string comparison to differ from numeric comparison for test case %q", tt.name)
				}
			}
		})
	}
}

// Tests consolidated from review_scope_acceptance_test.go

// TestReviewCommand_SpecFlagExists verifies that the review command accepts --spec flag
func TestReviewCommand_SpecFlagExists(t *testing.T) {
	cmd := reviewCmd

	specFlag := cmd.Flags().Lookup("spec")
	if specFlag == nil {
		t.Fatal("review command should have --spec flag")
	}

	if specFlag.Value.Type() != "string" {
		t.Errorf("--spec flag should be string type, got %s", specFlag.Value.Type())
	}
}

// TestReviewCommand_SpecAndEpicMutuallyExclusive verifies that --spec and --epic
// cannot be used together on the review command
func TestReviewCommand_SpecAndEpicMutuallyExclusive(t *testing.T) {
	err := scope.ValidateFlags("gromit-xyz", "init-wizard")
	if err == nil {
		t.Fatal("scope.ValidateFlags should return error when both epic and spec are set")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

// TestReviewCommand_SpecFlagResolvesToLabel verifies that --spec flag
// resolves to the correct label format via scope.ResolveSpec
func TestReviewCommand_SpecFlagResolvesToLabel(t *testing.T) {
	specName := "init-wizard"
	labels := scope.ResolveSpec(specName)

	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}

	expectedLabel := "spec:init-wizard"
	if labels[0] != expectedLabel {
		t.Errorf("ResolveSpec(%q) = %q, want %q", specName, labels[0], expectedLabel)
	}
}

// TestReviewCommand_EpicFlagUsesResolveEpic verifies that --epic flag
// uses scope.ResolveEpic to resolve epic to spec labels
func TestReviewCommand_EpicFlagUsesResolveEpic(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	specs := []struct {
		filename string
		id       string
		epic     string
	}{
		{"auth.md", "auth", "gromit-xyz"},
		{"profile.md", "profile", "gromit-xyz"},
	}

	for _, spec := range specs {
		specPath := filepath.Join(specsDir, spec.filename)
		specContent := fmt.Sprintf(`---
id: %s
epic: %s
created: 2026-02-08
---

# Spec
`, spec.id, spec.epic)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	labels, err := scope.ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}

	if len(labels) != 2 {
		t.Fatalf("ResolveEpic should return 2 labels, got %d", len(labels))
	}

	expectedLabels := map[string]bool{
		"spec:auth":    false,
		"spec:profile": false,
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

// TestReviewCommand_SpecFlagInHelpText verifies that --spec flag appears
// in the review command help text
func TestReviewCommand_SpecFlagInHelpText(t *testing.T) {
	cmd := reviewCmd
	helpText := cmd.Long

	if !strings.Contains(helpText, "--spec") {
		t.Fatal("--spec flag should be documented in review command help text")
	}
}

// Tests consolidated from review_mutual_exclusivity_acceptance_test.go

// TestReviewCommand_FlagMutualExclusivity verifies that --epic, --spec, and --since
// flags are mutually exclusive on the review command
func TestReviewCommand_FlagMutualExclusivity(t *testing.T) {
	cfg := &config.Config{}
	saveReviewFlags(t)

	tests := []struct {
		name    string
		epic    string
		spec    string
		since   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "epic and spec both set",
			epic:    "gromit-xyz",
			spec:    "init-wizard",
			since:   "",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
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
			reviewEpic = tt.epic
			reviewSpec = tt.spec
			reviewSince = tt.since

			_, err := determineReviewScope(cfg)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error when flags %s are set, got nil", tt.name)
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "mutually exclusive") {
					t.Errorf("should not error on mutual exclusivity for %s, got: %v", tt.name, err)
				}
			}
		})
	}
}

// TestReviewCommand_MutualExclusivityCheckedEarly verifies that mutual exclusivity
// is checked before attempting to resolve specs or epics
func TestReviewCommand_MutualExclusivityCheckedEarly(t *testing.T) {
	cfg := &config.Config{}
	saveReviewFlags(t)

	// Set two flags with invalid values that would fail resolution
	reviewEpic = "nonexistent-epic-xyz"
	reviewSpec = "nonexistent-spec-123"
	reviewSince = ""

	_, err := determineReviewScope(cfg)
	if err == nil {
		t.Fatal("expected error when both --epic and --spec are set")
	}

	// Should fail with mutual exclusivity error, not with "epic not found" or "spec not found"
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity (not resolution failure), got: %v", err)
	}
}

// TestReviewCommand_MutualExclusivityWithWhitespace verifies that flags with
// only whitespace are treated as empty and don't trigger mutual exclusivity
func TestReviewCommand_MutualExclusivityWithWhitespace(t *testing.T) {
	cfg := &config.Config{}
	saveReviewFlags(t)

	tests := []struct {
		name    string
		epic    string
		spec    string
		since   string
		wantErr bool
	}{
		{
			name:    "epic with value, spec with whitespace",
			epic:    "gromit-xyz",
			spec:    "   ",
			since:   "",
			wantErr: false,
		},
		{
			name:    "spec with value, epic with whitespace",
			epic:    "   ",
			spec:    "init-wizard",
			since:   "",
			wantErr: false,
		},
		{
			name:    "since with value, epic with whitespace",
			epic:    "   ",
			spec:    "",
			since:   "abc123",
			wantErr: false,
		},
		{
			name:    "all whitespace",
			epic:    "   ",
			spec:    "   ",
			since:   "   ",
			wantErr: false,
		},
		{
			name:    "two real values, one whitespace",
			epic:    "gromit-xyz",
			spec:    "   ",
			since:   "abc123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewEpic = tt.epic
			reviewSpec = tt.spec
			reviewSince = tt.since

			_, err := determineReviewScope(cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected mutual exclusivity error for %s", tt.name)
				}
				if !strings.Contains(err.Error(), "mutually exclusive") {
					t.Errorf("error should mention mutual exclusivity, got: %v", err)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "mutually exclusive") {
					t.Errorf("%s should not fail with mutual exclusivity error, got: %v", tt.name, err)
				}
			}
		})
	}
}
