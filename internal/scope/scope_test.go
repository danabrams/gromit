package scope

import (
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

// TestValidateFlags tests that ValidateFlags errors when both epic and spec flags are set
func TestValidateFlags(t *testing.T) {
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
