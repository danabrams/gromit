package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/experiment"
)

// TestOrchestrator_EmitsConvergenceSummaryToStderr verifies that when an experiment
// has been loaded, the orchestrator emits a summary line to stderr after starting the run.
func TestOrchestrator_EmitsConvergenceSummaryToStderr(t *testing.T) {
	t.Parallel(
	// Create experiments
	)

	exp := &experiment.Experiment{
		ID:    "exp-1",
		Phase: "build",
	}

	// Create an experiment manager with one experiment
	expMgr := experiment.NewManager([]*experiment.Experiment{exp}, "")

	getBead := func(_ context.Context) (*bead.Bead, error) { return nil, nil }

	cfg := OrchestratorConfig{
		Gate:          &fakeStage{},
		Build:         &fakeStage{},
		Validate:      &fakeStage{},
		Epilogue:      &fakeStage{},
		GetBead:       getBead,
		Config:        &config.Config{},
		ExperimentMgr: expMgr,
	}

	orchestrator := NewOrchestrator(cfg)
	emitter := orchestrator.GetEmitter()
	ch := emitter.Subscribe()
	defer emitter.Unsubscribe(ch)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := orchestrator.Run(ctx, 0, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() should not error: %v", err)
	}

	messages := collectLogMessages(t, ch, 100*time.Millisecond)
	if !containsAll(messages, []string{"Experiment", "converged"}) {
		t.Fatalf("Expected log messages to include convergence summary, got: %v", messages)
	}

}

func collectLogMessages(t *testing.T, ch <-chan events.Event, timeout time.Duration) []string {
	t.Helper()
	var messages []string
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return messages
			}
			if logEvt, ok := evt.(*events.LogEvent); ok {
				messages = append(messages, logEvt.Message)
			}
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(timeout)
		case <-timer.C:
			return messages
		}
	}
}

func containsAll(messages []string, substrings []string) bool {
	for _, substr := range substrings {
		found := false
		for _, msg := range messages {
			if strings.Contains(msg, substr) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
