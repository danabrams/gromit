package regression_test

import (
    "context"
    "errors"
    "strings"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/pipeline"
    regression "github.com/danabrams/gromit/internal/pipeline/qualitygate/regression"
    "github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestStagePassesWhenCommandSucceeds(t *testing.T) {
    runner := fakeRunner("ok", "", 0, nil)
    stage := regression.New(runner)
    input := pipeline.Input{
        Config: &config.Config{
            QualityGates: &config.QualityGatesConfig{
                Regression: &config.RegressionGateConfig{
                    Enabled: true,
                    Command: "go test ./...",
                },
            },
        },
    }

    out, err := stage.Run(context.Background(), input)
    if err != nil {
        t.Fatalf("Run() error = %v, want nil", err)
    }
    if out.Decision != pipeline.Proceed {
        t.Fatalf("Decision = %v, want Proceed", out.Decision)
    }
}

func TestStageBlocksWhenCommandFails(t *testing.T) {
    var capturedCommand string
    runner := fakeRunnerWithCapture("--- FAIL: TestFoo", "stderr", 1, nil, &capturedCommand)
    stage := regression.New(runner)

    input := pipeline.Input{
        Config: &config.Config{
            QualityGates: &config.QualityGatesConfig{
                Regression: &config.RegressionGateConfig{
                    Enabled: true,
                    Command: "go test ./...",
                },
            },
        },
    }

    out, err := stage.Run(context.Background(), input)
    if err != nil {
        t.Fatalf("Run() error = %v, want nil", err)
    }
    if out.Decision != pipeline.Block {
        t.Fatalf("Decision = %v, want Block", out.Decision)
    }
    if !strings.Contains(out.ValidationFailures[0], "FAIL") {
        t.Fatalf("ValidationFailures = %v, want test output", out.ValidationFailures)
    }
    if target := "go test ./..."; capturedCommand != target {
        t.Fatalf("command = %q, want %q", capturedCommand, target)
    }
}

func TestStageReturnsErrorWhenRunnerErrors(t *testing.T) {
    runner := fakeRunner("", "", 0, errors.New("boom"))
    stage := regression.New(runner)
    input := pipeline.Input{
        Config: &config.Config{
            QualityGates: &config.QualityGatesConfig{
                Regression: &config.RegressionGateConfig{
                    Enabled: true,
                    Command: "go test ./...",
                },
            },
        },
    }

    if _, err := stage.Run(context.Background(), input); err == nil {
        t.Fatalf("Run() error = nil, want non-nil")
    }
}

func fakeRunner(stdout, stderr string, exitCode int, err error) runtypes.CmdRunnerFn {
    return func(_ context.Context, _ string, _ string) (string, string, int, error) {
        return stdout, stderr, exitCode, err
    }
}

func fakeRunnerWithCapture(stdout, stderr string, exitCode int, err error, capturedCommand *string) runtypes.CmdRunnerFn {
    return func(_ context.Context, command string, _ string) (string, string, int, error) {
        if capturedCommand != nil {
            *capturedCommand = command
        }
        return stdout, stderr, exitCode, err
    }
}
