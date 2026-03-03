package wiring_test

import (
    "context"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/pipeline"
    wiringstage "github.com/danabrams/gromit/internal/pipeline/qualitygate/wiring"
)

func TestWiringGate_SkipsWhenDisabled(t *testing.T) {
    stage := wiringstage.New(func(context.Context) (string, error) {
        t.Fatal("git diff should not run when wiring gate is disabled")
        return "", nil
    })

    input := pipeline.Input{
        Config: &config.Config{
            WiringGate: config.WiringGateConfig{Enabled: false},
        },
    }

    output, err := stage.Run(context.Background(), input)
    if err != nil {
        t.Fatalf("Run() error = %v", err)
    }
    if output.Decision != pipeline.Proceed {
        t.Errorf("Decision = %v, want %v", output.Decision, pipeline.Proceed)
    }
    if len(output.WiringFailures) != 0 {
        t.Fatalf("WiringFailures = %v, want none", output.WiringFailures)
    }
}
