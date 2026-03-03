package regression_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	regression "github.com/danabrams/gromit/internal/pipeline/qualitygate/regression"
)

func TestStagePassesWhenCommandSucceeds(t *testing.T) {
	runner := newFakeRunner(
		runResponse{stdout: "github.com/danabrams/gromit\n", exitCode: 0},
		runResponse{stdout: "github.com/danabrams/gromit\n" +
			"github.com/danabrams/gromit/internal/foo\n", exitCode: 0},
		runResponse{stdout: "ok", exitCode: 0},
	)
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
	if len(runner.commands) != 3 {
		t.Fatalf("commands = %v, want 3", runner.commands)
	}
}

func TestStageBlocksWhenCommandFails(t *testing.T) {
	runner := newFakeRunner(
		runResponse{stdout: "github.com/danabrams/gromit\n", exitCode: 0},
		runResponse{stdout: "github.com/danabrams/gromit\ngithub.com/danabrams/gromit/internal/foo\n", exitCode: 0},
		runResponse{stdout: "--- FAIL: TestFoo", exitCode: 1},
	)
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
	if len(out.ValidationFailures) == 0 {
		t.Fatalf("ValidationFailures = %v, want at least one entry", out.ValidationFailures)
	}
	if !strings.Contains(out.ValidationFailures[0], "FAIL") {
		t.Fatalf("ValidationFailures = %v, want summary with FAIL", out.ValidationFailures)
	}
}

func TestStageReturnsErrorWhenRunnerErrors(t *testing.T) {
	runner := newFakeRunner(
		runResponse{err: errors.New("boom")},
	)
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

func TestStageFiltersTouchedPackages(t *testing.T) {
	runner := newFakeRunner(
		runResponse{stdout: "github.com/danabrams/gromit\n", exitCode: 0},
		runResponse{stdout: strings.Join([]string{
			"github.com/danabrams/gromit",
			"github.com/danabrams/gromit/internal/foo",
			"github.com/danabrams/gromit/internal/bar",
		}, "\n") + "\n", exitCode: 0},
		runResponse{stdout: "ok", exitCode: 0},
	)
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
		TouchedPackages: []string{"internal/foo"},
	}

	out, err := stage.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Fatalf("Decision = %v, want Proceed", out.Decision)
	}

	last := runner.commands[len(runner.commands)-1]
	if strings.Contains(last, "internal/foo") {
		t.Fatalf("Last command = %q should not mention touched packages", last)
	}
	if !strings.Contains(last, "./internal/bar") {
		t.Fatalf("Last command = %q should include remaining package", last)
	}
}

func TestStageSkipsWhenTargetsExhausted(t *testing.T) {
	runner := newFakeRunner(
		runResponse{stdout: "github.com/danabrams/gromit\n", exitCode: 0},
		runResponse{stdout: strings.Join([]string{
			"github.com/danabrams/gromit",
			"github.com/danabrams/gromit/internal/foo",
		}, "\n") + "\n", exitCode: 0},
	)
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
		TouchedPackages: []string{"internal/foo", "."},
	}

	out, err := stage.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Fatalf("Decision = %v, want Proceed", out.Decision)
	}
	if len(runner.commands) != 2 {
		t.Fatalf("commands = %v, want only listing commands", runner.commands)
	}
}

type fakeRunner struct {
	responses []runResponse
	commands  []string
}

type runResponse struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func newFakeRunner(responses ...runResponse) *fakeRunner {
	return &fakeRunner{responses: append([]runResponse(nil), responses...)}
}

func (f *fakeRunner) Run(ctx context.Context, command, workDir string) (string, string, int, error) {
	if len(f.responses) == 0 {
		return "", "", 0, errors.New("no response configured")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	f.commands = append(f.commands, command)
	return resp.stdout, resp.stderr, resp.exitCode, resp.err
}
