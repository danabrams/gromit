package stages

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

// TestScenario_IOLeakDetected_Blocked verifies that when go test output
// contains "Test I/O incomplete" or "WaitDelay expired", the validate stage
// classifies it as an infrastructure blocker (not a replan-worthy test failure).
// This prevents the agent from thrashing on fix tasks for lifecycle bugs.
func TestScenario_IOLeakDetected_Blocked(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantSubstr string
	}{
		{
			name: "test_io_incomplete",
			output: `ok  	github.com/example/pkg1	0.5s
FAIL	github.com/example/pkg2	30.1s
panic: test timed out after 30s
Test I/O incomplete
FAIL	github.com/example/pkg2	30.1s`,
			wantSubstr: "Test I/O incomplete",
		},
		{
			name: "waitdelay_expired",
			output: `FAIL	github.com/example/pkg3	30.1s
exec: WaitDelay expired before I/O complete
FAIL`,
			wantSubstr: "WaitDelay expired",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := &fakeValidator{
				result: validator.FinalResult{
					Pass: false,
					AlwaysRun: validator.CheckResults{
						Results: []validator.CheckResult{
							{Name: "go test ./...", Pass: false, Output: tc.output},
						},
					},
					ProjectChecks: validator.CheckResults{
						Results: []validator.CheckResult{},
					},
				},
			}

			stage := NewValidateStage(v, ValidateStageConfig{
				WorkDir: "/tmp/work",
			}, nil, nil, nil)

			rs := runstore.NewRunState("spec-io", "proj-001")

			action, err := stage.Run(context.Background(), rs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Assert: classified as Blocked, not ReplanFrom
			if action.Kind != specloop.Blocked {
				t.Fatalf("expected Blocked, got %v", action.Kind)
			}

			// Assert: BlockerSummary starts with "infrastructure_io_leak:"
			if !strings.HasPrefix(rs.BlockerSummary, "infrastructure_io_leak:") {
				t.Fatalf("expected BlockerSummary to start with 'infrastructure_io_leak:', got %q", rs.BlockerSummary)
			}

			// Assert: BlockerSummary contains the diagnostic message
			if !strings.Contains(rs.BlockerSummary, "leaked subprocess I/O") {
				t.Fatalf("expected BlockerSummary to contain 'leaked subprocess I/O', got %q", rs.BlockerSummary)
			}

			// Assert: BlockerSummary contains the check name
			if !strings.Contains(rs.BlockerSummary, "go test ./...") {
				t.Fatalf("expected BlockerSummary to contain check name, got %q", rs.BlockerSummary)
			}
		})
	}
}

// TestScenario_IOLeakNotPresent_Replans verifies that normal test failures
// (without I/O leak signatures) are still classified as ReplanFrom.
func TestScenario_IOLeakNotPresent_Replans(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{
						Name: "go test ./...",
						Pass: false,
						Output: `--- FAIL: TestSomething (0.01s)
    foo_test.go:42: expected 1, got 2
FAIL	github.com/example/pkg1	0.5s`,
					},
				},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: "/tmp/work",
	}, nil, nil, nil)

	rs := runstore.NewRunState("spec-normal", "proj-001")

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: classified as ReplanFrom (normal test failure)
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}

	// Assert: no infrastructure blocker set
	if rs.BlockerSummary != "" {
		t.Fatalf("expected empty BlockerSummary, got %q", rs.BlockerSummary)
	}
}

// TestScenario_IOLeakInProjectCheck_Blocked verifies that I/O leak detection
// also works for project checks (not just always-run checks).
func TestScenario_IOLeakInProjectCheck_Blocked(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "go test ./...", Pass: true, Output: "ok"},
				},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{
					{
						Name: "integration tests",
						Pass: false,
						Output: `FAIL	github.com/example/integration	60.1s
Test I/O incomplete
exec: WaitDelay expired before I/O complete`,
					},
				},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir: "/tmp/work",
	}, nil, nil, nil)

	rs := runstore.NewRunState("spec-proj", "proj-001")

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Assert: classified as Blocked
	if action.Kind != specloop.Blocked {
		t.Fatalf("expected Blocked, got %v", action.Kind)
	}

	// Assert: references the project check name
	if !strings.Contains(rs.BlockerSummary, "integration tests") {
		t.Fatalf("expected BlockerSummary to contain 'integration tests', got %q", rs.BlockerSummary)
	}
}
