package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

type fakeValidator struct {
	result validator.FinalResult
	err    error
}

func (f *fakeValidator) RunFinal(ctx context.Context, alwaysRun []validator.Check, projectChecks []validator.Check, workDir string) (validator.FinalResult, error) {
	return f.result, f.err
}

// Verify ValidateStage satisfies the Stage interface.
var _ specloop.Stage = (*ValidateStage)(nil)

func TestValidateStage_AllPass_Continue(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: true,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "test", Pass: true}},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil)

	if stage.Name() != "validate" {
		t.Fatalf("expected name 'validate', got %q", stage.Name())
	}

	rs := runstore.NewRunState("spec-001", "proj-001")
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true")
	}
}

func TestValidateStage_Failure_ReplanFrom(t *testing.T) {
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "test", Pass: false, Output: "FAIL"}},
			},
			ProjectChecks: validator.CheckResults{
				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
			},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Cycle = 1
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be false")
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) == 0 {
		t.Fatal("expected failures to be non-empty")
	}
}
