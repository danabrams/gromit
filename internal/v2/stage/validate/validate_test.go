package validate

import (
    "context"
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

type spyValidationRunner struct {
    commands []string
}

func (s *spyValidationRunner) Run(_ context.Context, command string) error {
    s.commands = append(s.commands, command)
    return nil
}
