package scope

import (
	"strings"
	"testing"
)

// TestValidateFlags_ThreeWayMutualExclusivity tests that ValidateFlags enforces
// mutual exclusivity between epic, spec, and since parameters
func TestValidateFlags_ThreeWayMutualExclusivity(t *testing.T) {
	tests := []struct {
		name    string
		epic    string
		spec    string
		since   string
		wantErr bool
		errMsg  string
	}{
		// Valid cases - only one flag or none
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
			name:    "none set",
			epic:    "",
			spec:    "",
			since:   "",
			wantErr: false,
		},
		// Invalid cases - two flags
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
		// Invalid case - all three flags
		{
			name:    "all three flags set",
			epic:    "gromit-xyz",
			spec:    "init-wizard",
			since:   "abc123",
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		// Whitespace handling
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
			errMsg:  "mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: ValidateFlags needs to be updated to accept three parameters
			err := ValidateFlags(tt.epic, tt.spec, tt.since)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFlags(%q, %q, %q) error = %v, wantErr %v",
					tt.epic, tt.spec, tt.since, err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateFlags(%q, %q, %q) error = %q, want error containing %q",
						tt.epic, tt.spec, tt.since, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestValidateFlags_ThreeWayErrorMessage verifies that error messages
// are clear when multiple flags are set
func TestValidateFlags_ThreeWayErrorMessage(t *testing.T) {
	tests := []struct {
		name  string
		epic  string
		spec  string
		since string
	}{
		{
			name:  "epic and spec",
			epic:  "gromit-xyz",
			spec:  "init-wizard",
			since: "",
		},
		{
			name:  "epic and since",
			epic:  "gromit-xyz",
			spec:  "",
			since: "abc123",
		},
		{
			name:  "spec and since",
			epic:  "",
			spec:  "init-wizard",
			since: "abc123",
		},
		{
			name:  "all three",
			epic:  "gromit-xyz",
			spec:  "init-wizard",
			since: "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.epic, tt.spec, tt.since)
			if err == nil {
				t.Fatalf("ValidateFlags should return error for %s", tt.name)
			}

			errMsg := err.Error()
			// Error message should indicate mutual exclusivity
			if !strings.Contains(strings.ToLower(errMsg), "mutually exclusive") {
				t.Errorf("error message should mention 'mutually exclusive', got: %q", errMsg)
			}
		})
	}
}

// TestValidateFlags_OnlyOneParameterAllowed verifies that ValidateFlags
// allows at most one non-empty parameter
func TestValidateFlags_OnlyOneParameterAllowed(t *testing.T) {
	// Test all valid single-flag combinations
	validCombinations := []struct {
		epic  string
		spec  string
		since string
	}{
		{"gromit-xyz", "", ""},
		{"", "init-wizard", ""},
		{"", "", "abc123"},
		{"", "", ""},
	}

	for _, tc := range validCombinations {
		err := ValidateFlags(tc.epic, tc.spec, tc.since)
		if err != nil {
			t.Errorf("ValidateFlags(%q, %q, %q) should not return error, got: %v",
				tc.epic, tc.spec, tc.since, err)
		}
	}

	// Test all invalid multi-flag combinations
	invalidCombinations := []struct {
		epic  string
		spec  string
		since string
	}{
		{"gromit-xyz", "init-wizard", ""},
		{"gromit-xyz", "", "abc123"},
		{"", "init-wizard", "abc123"},
		{"gromit-xyz", "init-wizard", "abc123"},
	}

	for _, tc := range invalidCombinations {
		err := ValidateFlags(tc.epic, tc.spec, tc.since)
		if err == nil {
			t.Errorf("ValidateFlags(%q, %q, %q) should return error",
				tc.epic, tc.spec, tc.since)
		}
		if !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("ValidateFlags(%q, %q, %q) error should mention mutual exclusivity, got: %v",
				tc.epic, tc.spec, tc.since, err)
		}
	}
}

// TestValidateFlags_TrimsWhitespace verifies that ValidateFlags considers
// trimmed values when checking mutual exclusivity
func TestValidateFlags_TrimsWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		epic    string
		spec    string
		since   string
		wantErr bool
	}{
		{
			name:    "leading/trailing spaces in epic",
			epic:    "  gromit-xyz  ",
			spec:    "",
			since:   "",
			wantErr: false,
		},
		{
			name:    "leading/trailing spaces in spec",
			epic:    "",
			spec:    "  init-wizard  ",
			since:   "",
			wantErr: false,
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
			name:    "whitespace-only epic with real spec",
			epic:    "   ",
			spec:    "init-wizard",
			since:   "",
			wantErr: false,
		},
		{
			name:    "whitespace-only spec with real epic",
			epic:    "gromit-xyz",
			spec:    "   ",
			since:   "",
			wantErr: false,
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

// TestValidateFlags_MentionsFlags verifies that error messages mention
// which flags are conflicting
func TestValidateFlags_MentionsFlags(t *testing.T) {
	tests := []struct {
		name         string
		epic         string
		spec         string
		since        string
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

// TestValidateFlags_CountsNonEmptyFlags verifies that ValidateFlags correctly
// counts non-empty flags (after trimming whitespace)
func TestValidateFlags_CountsNonEmptyFlags(t *testing.T) {
	tests := []struct {
		name          string
		epic          string
		spec          string
		since         string
		expectedCount int
		wantErr       bool
	}{
		{
			name:          "zero non-empty flags",
			epic:          "",
			spec:          "",
			since:         "",
			expectedCount: 0,
			wantErr:       false,
		},
		{
			name:          "one non-empty flag (epic)",
			epic:          "gromit-xyz",
			spec:          "",
			since:         "",
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name:          "one non-empty flag (spec)",
			epic:          "",
			spec:          "init-wizard",
			since:         "",
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name:          "one non-empty flag (since)",
			epic:          "",
			spec:          "",
			since:         "abc123",
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name:          "two non-empty flags",
			epic:          "gromit-xyz",
			spec:          "init-wizard",
			since:         "",
			expectedCount: 2,
			wantErr:       true,
		},
		{
			name:          "three non-empty flags",
			epic:          "gromit-xyz",
			spec:          "init-wizard",
			since:         "abc123",
			expectedCount: 3,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.epic, tt.spec, tt.since)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFlags with %d non-empty flags: error = %v, wantErr %v",
					tt.expectedCount, err, tt.wantErr)
			}
		})
	}
}
