package main

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestReviewCommand_FlagMutualExclusivity verifies that --epic, --spec, and --since
// flags are mutually exclusive on the review command
func TestReviewCommand_FlagMutualExclusivity(t *testing.T) {
	cfg := &config.Config{}

	// Save original flag values and restore them after test
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	defer func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	}()

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

			// Check if error matches expectation
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error when flags %s are set, got nil", tt.name)
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				// For non-error cases, we might still get errors due to missing
				// state files, specs, or epics, but those errors should NOT be
				// about mutual exclusivity
				if err != nil && strings.Contains(err.Error(), "mutually exclusive") {
					t.Errorf("should not error on mutual exclusivity for %s, got: %v", tt.name, err)
				}
			}
		})
	}
}

// TestReviewCommand_SinceAndEpicMutuallyExclusive verifies that --since and --epic
// cannot be used together
func TestReviewCommand_SinceAndEpicMutuallyExclusive(t *testing.T) {
	cfg := &config.Config{}

	// Save original flag values and restore them after test
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	defer func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	}()

	reviewEpic = "gromit-xyz"
	reviewSpec = ""
	reviewSince = "abc123"

	_, err := determineReviewScope(cfg)
	if err == nil {
		t.Fatal("expected error when both --since and --epic are set")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

// TestReviewCommand_SinceAndSpecMutuallyExclusive verifies that --since and --spec
// cannot be used together
func TestReviewCommand_SinceAndSpecMutuallyExclusive(t *testing.T) {
	cfg := &config.Config{}

	// Save original flag values and restore them after test
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	defer func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	}()

	reviewEpic = ""
	reviewSpec = "init-wizard"
	reviewSince = "abc123"

	_, err := determineReviewScope(cfg)
	if err == nil {
		t.Fatal("expected error when both --since and --spec are set")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

// TestReviewCommand_AllThreeFlagsMutuallyExclusive verifies that using
// all three flags together is an error
func TestReviewCommand_AllThreeFlagsMutuallyExclusive(t *testing.T) {
	cfg := &config.Config{}

	// Save original flag values and restore them after test
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	defer func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	}()

	reviewEpic = "gromit-xyz"
	reviewSpec = "init-wizard"
	reviewSince = "abc123"

	_, err := determineReviewScope(cfg)
	if err == nil {
		t.Fatal("expected error when all three flags are set")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

// TestReviewCommand_OnlyOneFlagAllowedAtATime verifies that exactly one
// scope flag can be set at a time, or none
func TestReviewCommand_OnlyOneFlagAllowedAtATime(t *testing.T) {
	cfg := &config.Config{}

	// Save original flag values and restore them after test
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	defer func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	}()

	// These combinations should be allowed (only one flag set or none)
	allowedCombinations := []struct {
		name  string
		epic  string
		spec  string
		since string
	}{
		{name: "only epic", epic: "gromit-xyz", spec: "", since: ""},
		{name: "only spec", epic: "", spec: "init-wizard", since: ""},
		{name: "only since", epic: "", spec: "", since: "abc123"},
		{name: "none set", epic: "", spec: "", since: ""},
	}

	for _, tc := range allowedCombinations {
		t.Run(tc.name, func(t *testing.T) {
			reviewEpic = tc.epic
			reviewSpec = tc.spec
			reviewSince = tc.since

			_, err := determineReviewScope(cfg)
			// Should not fail with mutual exclusivity error
			if err != nil && strings.Contains(err.Error(), "mutually exclusive") {
				t.Errorf("%s should not fail with mutual exclusivity error, got: %v", tc.name, err)
			}
		})
	}

	// These combinations should be rejected (more than one flag set)
	rejectedCombinations := []struct {
		name  string
		epic  string
		spec  string
		since string
	}{
		{name: "epic and spec", epic: "gromit-xyz", spec: "init-wizard", since: ""},
		{name: "epic and since", epic: "gromit-xyz", spec: "", since: "abc123"},
		{name: "spec and since", epic: "", spec: "init-wizard", since: "abc123"},
		{name: "all three", epic: "gromit-xyz", spec: "init-wizard", since: "abc123"},
	}

	for _, tc := range rejectedCombinations {
		t.Run(tc.name, func(t *testing.T) {
			reviewEpic = tc.epic
			reviewSpec = tc.spec
			reviewSince = tc.since

			_, err := determineReviewScope(cfg)
			if err == nil {
				t.Fatalf("%s should fail with mutual exclusivity error", tc.name)
			}
			if !strings.Contains(err.Error(), "mutually exclusive") {
				t.Errorf("%s should fail with mutual exclusivity error, got: %v", tc.name, err)
			}
		})
	}
}

// TestReviewCommand_MutualExclusivityCheckedEarly verifies that mutual exclusivity
// is checked before attempting to resolve specs or epics
func TestReviewCommand_MutualExclusivityCheckedEarly(t *testing.T) {
	cfg := &config.Config{}

	// Save original flag values and restore them after test
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	defer func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	}()

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

	// Save original flag values and restore them after test
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	defer func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	}()

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
				// Should not fail with mutual exclusivity error
				if err != nil && strings.Contains(err.Error(), "mutually exclusive") {
					t.Errorf("%s should not fail with mutual exclusivity error, got: %v", tt.name, err)
				}
			}
		})
	}
}
