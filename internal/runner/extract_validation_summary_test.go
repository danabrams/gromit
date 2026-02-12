package runner

import (
	"strings"
	"testing"
)

func TestExtractValidationSummary_GoTestFailures(t *testing.T) {
	// Expected failure: extractValidationSummary function does not exist yet

	tests := []struct {
		name          string
		input         string
		wantContains  []string
		wantNotContain []string
	}{
		{
			name: "single test failure with FAIL line",
			input: `=== RUN   TestFoo
--- FAIL: TestFoo (0.01s)
    foo_test.go:15: expected 1, got 2
FAIL	github.com/example/pkg	0.023s
FAIL`,
			wantContains: []string{
				"--- FAIL: TestFoo",
				"FAIL\tgithub.com/example/pkg",
			},
		},
		{
			name: "multiple test failures",
			input: `=== RUN   TestAlpha
--- FAIL: TestAlpha (0.01s)
    alpha_test.go:10: assertion failed
=== RUN   TestBeta
--- FAIL: TestBeta (0.02s)
    beta_test.go:20: wrong result
FAIL	github.com/example/alpha	0.050s
FAIL`,
			wantContains: []string{
				"--- FAIL: TestAlpha",
				"--- FAIL: TestBeta",
				"FAIL\tgithub.com/example/alpha",
			},
		},
		{
			name: "subtest failures",
			input: `=== RUN   TestParent
=== RUN   TestParent/subcase_one
--- FAIL: TestParent/subcase_one (0.00s)
    parent_test.go:25: subcase one failed
=== RUN   TestParent/subcase_two
--- PASS: TestParent/subcase_two (0.00s)
--- FAIL: TestParent (0.01s)
FAIL	github.com/example/pkg	0.015s`,
			wantContains: []string{
				"--- FAIL: TestParent/subcase_one",
				"--- FAIL: TestParent",
			},
			wantNotContain: []string{
				"--- PASS:",
				"=== RUN",
			},
		},
		{
			name:  "passing tests produce empty summary",
			input: `ok  	github.com/example/pkg	0.010s`,
			wantContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractValidationSummary(tt.input)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("extractValidationSummary() missing expected substring %q\ngot:\n%s", want, result)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(result, notWant) {
					t.Errorf("extractValidationSummary() should not contain %q\ngot:\n%s", notWant, result)
				}
			}

			if len(tt.wantContains) == 0 && result != "" {
				t.Errorf("expected empty summary for passing tests, got %q", result)
			}
		})
	}
}

func TestExtractValidationSummary_GoVetDiagnostics(t *testing.T) {
	// Expected failure: extractValidationSummary function does not exist yet

	tests := []struct {
		name         string
		input        string
		wantContains []string
	}{
		{
			name: "unused variable diagnostic",
			input: `# github.com/example/pkg
./file.go:10:6: x declared and not used
FAIL	github.com/example/pkg [build failed]`,
			wantContains: []string{
				"x declared and not used",
			},
		},
		{
			name: "multiple vet diagnostics",
			input: `# github.com/example/pkg
./main.go:5:2: imported and not used: "fmt"
./main.go:12:6: y declared and not used
./util.go:8:2: unreachable code
FAIL	github.com/example/pkg [build failed]`,
			wantContains: []string{
				"imported and not used",
				"declared and not used",
				"unreachable code",
			},
		},
		{
			name: "printf format diagnostic",
			input: `# github.com/example/pkg
./server.go:42:14: fmt.Sprintf format %d has arg of wrong type string
FAIL	github.com/example/pkg [vet]`,
			wantContains: []string{
				"fmt.Sprintf format %d has arg of wrong type string",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractValidationSummary(tt.input)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("extractValidationSummary() missing expected substring %q\ngot:\n%s", want, result)
				}
			}
		})
	}
}

func TestExtractValidationSummary_CappedAt500Chars(t *testing.T) {
	// Expected failure: extractValidationSummary function does not exist yet

	// Build a long output with many test failures that would exceed 500 chars
	var builder strings.Builder
	for i := 0; i < 50; i++ {
		builder.WriteString("--- FAIL: TestLongNameThatAddsUpToExceedTheCharLimit_")
		builder.WriteString(strings.Repeat("x", 20))
		builder.WriteString(" (0.01s)\n")
		builder.WriteString("    test.go:10: some failure message\n")
	}
	builder.WriteString("FAIL\tgithub.com/example/pkg\t1.234s\n")

	input := builder.String()
	result := extractValidationSummary(input)

	if len(result) > 500 {
		t.Errorf("extractValidationSummary() result length %d exceeds 500 char cap", len(result))
	}

	// Even capped, the result should still contain useful failure info
	if result == "" {
		t.Error("expected non-empty summary even with many failures")
	}
}

func TestExtractValidationSummary_EmptyInput(t *testing.T) {
	// Expected failure: extractValidationSummary function does not exist yet

	result := extractValidationSummary("")
	if result != "" {
		t.Errorf("expected empty summary for empty input, got %q", result)
	}
}

func TestExtractValidationSummary_MixedGoTestAndVetOutput(t *testing.T) {
	// Expected failure: extractValidationSummary function does not exist yet

	input := `=== RUN   TestHandler
--- FAIL: TestHandler (0.03s)
    handler_test.go:45: response code mismatch
# github.com/example/pkg
./handler.go:20:6: unusedVar declared and not used
FAIL	github.com/example/pkg	0.045s`

	result := extractValidationSummary(input)

	// Should extract both test failure lines and vet diagnostics
	if !strings.Contains(result, "--- FAIL: TestHandler") {
		t.Errorf("expected test failure line in summary, got:\n%s", result)
	}
	if !strings.Contains(result, "unusedVar declared and not used") {
		t.Errorf("expected vet diagnostic in summary, got:\n%s", result)
	}
}
