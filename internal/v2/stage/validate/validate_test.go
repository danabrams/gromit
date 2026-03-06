package validate

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestStageRunsConfiguredCommands(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Validation.Commands = []string{"cmd-one", "cmd-two"}

	runner := &spyValidationRunner{}
	stage, err := New(cfg, runner)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	res, err := stage.Run(context.Background(), &stagepkg.Request{Config: cfg})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil || res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("unexpected decision: %v", res)
	}

	if !reflect.DeepEqual(runner.commands, cfg.Validation.Commands) {
		t.Fatalf("commands executed = %v, want %v", runner.commands, cfg.Validation.Commands)
	}
}

func TestStageStopsAtFirstFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Validation.Commands = []string{"cmd-one", "cmd-two"}

	failure := fmt.Errorf("validation failure")
	runner := &failingValidationRunner{failOn: "cmd-one", failErr: failure}
	stage, err := New(cfg, runner)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	res, err := stage.Run(context.Background(), &stagepkg.Request{Config: cfg})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil || res.Decision != stagepkg.DecisionFail {
		t.Fatalf("unexpected decision: %v", res)
	}

	artifacts, ok := res.Artifacts.(*ValidateArtifacts)
	if !ok {
		t.Fatalf("expected ValidateArtifacts, got %T", res.Artifacts)
	}
	if artifacts.FailedCommand != "cmd-one" {
		t.Fatalf("failed command = %q, want cmd-one", artifacts.FailedCommand)
	}
	if artifacts.FailureError != failure {
		t.Fatalf("failure error = %v, want %v", artifacts.FailureError, failure)
	}

	if len(runner.commands) != 1 {
		t.Fatalf("commands executed = %v, want first command only", runner.commands)
	}
}

type spyValidationRunner struct {
	commands []string
}

func (s *spyValidationRunner) Run(_ context.Context, command string) error {
	s.commands = append(s.commands, command)
	return nil
}

type failingValidationRunner struct {
	commands []string
	failOn   string
	failErr  error
}

func (s *failingValidationRunner) Run(_ context.Context, command string) error {
	s.commands = append(s.commands, command)
	if command == s.failOn {
		return s.failErr
	}
	return nil
}
