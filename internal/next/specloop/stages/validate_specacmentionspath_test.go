package stages

import (
	"testing"
)

// TestUnit_SpecACMentionsPath tests the specACMentionsPath function across various
// scenarios, including the critical case where "Acceptance Criteria" appears in prose
// (e.g., in Vision) before the real ## Acceptance Criteria heading.
func TestUnit_SpecACMentionsPath(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		filePath string
		want     bool
		desc     string
	}{
		{
			name: "file in AC section, with prose mention before heading",
			spec: `# Spec 0004h

## Vision
The Acceptance Criteria describe the expected behavior. When implementing this,
follow the Acceptance Criteria carefully. These Acceptance Criteria ensure quality.

## Acceptance Criteria

1. Keep tests in write_contracts_test.go
2. All assertions must pass

## Scenarios
...
`,
			filePath: "write_contracts_test.go",
			want:     true, // Should match the AC section, not the Vision prose
			desc:     "Verifies that a file listed in the real AC section is found even when 'Acceptance Criteria' appears in prose before the heading",
		},
		{
			name: "prose mention only before AC heading not in AC",
			spec: `# Spec 0004h

## Vision
This change replaces old_file_test.go with a new implementation.

## Acceptance Criteria

1. Keep tests in write_contracts_test.go
2. Implement the new_file_test.go pattern

## Scenarios
...
`,
			filePath: "old_file_test.go",
			want:     false, // Should return false because old_file_test.go is only mentioned in Vision, not in AC
			desc:     "Verifies that filename mentioned only in pre-AC prose doesn't match when not in real AC section",
		},
		{
			name: "prose mention in Non-goals before real AC heading",
			spec: `# Spec 0004h

## Vision
Some vision...

## Non-goals
Do not modify Acceptance Criteria in the test files.

## Acceptance Criteria

1. Ensure pattern_test.go contains TestPattern
2. Do not touch other files

## Scenarios
...
`,
			filePath: "pattern_test.go",
			want:     true, // Should match real AC, not Non-goals prose
			desc:     "Verifies that prose mention in Non-goals doesn't prevent matching real AC",
		},
		{
			name: "file not mentioned in AC despite prose mentions elsewhere",
			spec: `# Spec 0004h

## Vision
The Acceptance Criteria for this component are strict.

## Acceptance Criteria

1. Keep tests in write_contracts_test.go
2. Update primary_test.go only

## Scenarios
...
`,
			filePath: "secondary_test.go",
			want:     false, // secondary_test.go not in real AC
			desc:     "Verifies that prose doesn't affect what's actually matched in AC",
		},
		{
			name: "multiple prose mentions before real AC",
			spec: `# Spec 0004h

## Vision
Follow the Acceptance Criteria when implementing. The Acceptance Criteria must be satisfied.

## Non-goals
Don't violate Acceptance Criteria elsewhere.

## Notes
The Acceptance Criteria specify the exact behavior needed.

## Acceptance Criteria

1. Tests must remain in stages_test.go
2. The correction should be accepted when AC doesn't mention the original file

## Scenarios
...
`,
			filePath: "stages_test.go",
			want:     true, // Should match real AC section
			desc:     "Verifies robustness when 'Acceptance Criteria' appears multiple times in prose",
		},
		{
			name: "empty file path",
			spec: `# Spec
## Acceptance Criteria
1. Keep tests in test.go
`,
			filePath: "",
			want:     false,
			desc:     "Verifies empty filePath always returns false",
		},
		{
			name: "no AC section in spec",
			spec: `# Spec 0004h

## Vision
Some vision...

## Non-goals
Some non-goals...
`,
			filePath: "test.go",
			want:     false,
			desc:     "Verifies return false when no ## Acceptance Criteria heading exists",
		},
		{
			name: "normal case: basename match in AC",
			spec: `# Spec

## Acceptance Criteria

1. Keep tests in write_contracts_test.go
`,
			filePath: "internal/next/specloop/stages/write_contracts_test.go",
			want:     true,
			desc:     "Verifies basename matching works when no prose confuses the issue",
		},
		{
			name: "AC section ends at next ## heading",
			spec: `# Spec

## Acceptance Criteria

1. Tests in main_test.go must pass

## Scenarios

Only unrelated content appears here after the boundary.
`,
			filePath: "main_test.go",
			want:     true, // Match is in AC section before ## Scenarios
			desc:     "Verifies AC section boundary stops at next ## heading",
		},
		{
			name: "prose mention in AC section should still match",
			spec: `# Spec

## Acceptance Criteria

The Acceptance Criteria require that fixture_test.go remains unchanged.
Follow the Acceptance Criteria closely.

1. fixture_test.go must have all tests

## Scenarios
`,
			filePath: "fixture_test.go",
			want:     true, // Should match because it's in AC section (prose or not)
			desc:     "Verifies prose within AC section still allows matching",
		},
		{
			name: "file_only_after_Scenarios_boundary",
			spec: `# Spec

## Acceptance Criteria

1. Tests in write_contracts_test.go must pass

## Scenarios

The post_scenarios_test.go file should not match because it appears only after Scenarios.
`,
			filePath: "post_scenarios_test.go",
			want:     false, // Should not match because it only appears after ## Scenarios
			desc:     "Verifies that filenames appearing only after ## Scenarios heading are not matched",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := specACMentionsPath(tt.spec, tt.filePath)
			if got != tt.want {
				t.Errorf("specACMentionsPath() = %v, want %v\n%s", got, tt.want, tt.desc)
			}
		})
	}
}
