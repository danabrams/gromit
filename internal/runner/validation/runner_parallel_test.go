package validation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
)

func TestRunCommandsParallelCancelsOnFirstFailure(t *testing.T) {
	cfg := &config.Config{Validation: config.ValidationConfig{MaxParallelCommands: 3}}
	slowStarted := make(chan struct{})
	slowCanceled := make(chan struct{})

	cmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		switch command {
		case "fast":
			return "", "", 1, errors.New("boom")
		case "slow":
			close(slowStarted)
			select {
			case <-ctx.Done():
				close(slowCanceled)
				return "", "", 0, ctx.Err()
			case <-time.After(5 * time.Second):
				return "", "", 0, errors.New("slow command ran to completion")
			}
		default:
			return "", "", 0, nil
		}
	}

	r := NewRunner(cfg, cmdRunner, nil, nil)
	ctx := context.Background()

	results, err := r.runCommandsParallel(ctx, []string{"fast", "slow"}, "", cfg.Validation.MaxParallelCommands)
	if err != nil {
		t.Fatalf("runCommandsParallel failure: %v", err)
	}
	select {
	case <-slowStarted:
		select {
		case <-slowCanceled:
		case <-time.After(50 * time.Millisecond):
			t.Fatalf("slow command was not canceled after fast failure")
		}
	default:
	}

	if len(results) != 2 {
		t.Fatalf("unexpected result count: %d", len(results))
	}
	if results[0].exitCode == 0 || results[0].err == nil {
		t.Fatalf("expected fast command to fail; got %+v", results[0])
	}
	if !errors.Is(results[1].err, context.Canceled) {
		t.Fatalf("expected slow command to be canceled; got %+v", results[1])
	}
}
