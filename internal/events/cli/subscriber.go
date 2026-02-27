package cli

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/runner/execution"
)

// CLISubscriber consumes events and produces formatted terminal output.
type CLISubscriber struct {
	output  execution.OverwriteWriter
	emitter *events.Emitter
	mu      sync.Mutex
}

// NewCLISubscriber creates a new CLI subscriber that writes to the given output writer.
func NewCLISubscriber(output interface{}, emitter *events.Emitter) *CLISubscriber {
	// Convert io.Writer to OverwriteWriter if needed
	var ow execution.OverwriteWriter
	switch w := output.(type) {
	case execution.OverwriteWriter:
		ow = w
	case io.Writer:
		ow = &basicWriter{w}
	default:
		ow = &basicWriter{io.Discard}
	}

	return &CLISubscriber{
		output:  ow,
		emitter: emitter,
	}
}

// basicWriter wraps an io.Writer to implement OverwriteWriter for backward compatibility.
type basicWriter struct {
	w io.Writer
}

func (bw *basicWriter) Write(p []byte) (int, error) {
	return bw.w.Write(p)
}

func (bw *basicWriter) WriteOverwrite(p []byte) (int, error) {
	// For basic writer, just write as-is
	return bw.w.Write(p)
}

// Start consumes events from the emitter until the context is cancelled or the emitter is closed.
// It blocks until the context is done.
func (c *CLISubscriber) Start(ctx context.Context) error {
	if c.emitter == nil {
		return fmt.Errorf("emitter is nil")
	}

	ch := c.emitter.Subscribe()
	defer c.emitter.Unsubscribe(ch)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				// Channel closed, emitter is done
				return nil
			}
			c.handleEvent(event)
		case <-ctx.Done():
			// Context cancelled
			return nil
		}
	}
}

// handleEvent processes a single event and writes output if applicable.
func (c *CLISubscriber) handleEvent(event events.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch e := event.(type) {
	case *events.RunStartEvent:
		fmt.Fprintf(c.output, "=== Starting run (max %d iterations, %v budget) ===\n", e.MaxIterations, e.TimeBudget)
	case *events.RunCompleteEvent:
		fmt.Fprintf(c.output, "=== Run complete: %s (%d iterations) ===\n", e.Reason, e.IterationsCompleted)
	case *events.IterationStartEvent:
		fmt.Fprintf(c.output, "=== Iteration %d: %s (%s) ===\n", e.Iteration, e.BeadTitle, e.BeadID)
	case *events.IterationCompleteEvent:
		status := "PASS"
		if !e.Success {
			status = "FAIL"
		}
		fmt.Fprintf(c.output, "=== Iteration %d: %s (%v) ===\n", e.Iteration, status, e.Duration)
	case *events.BeadCompleteEvent:
		fmt.Fprintf(c.output, "=== Bead complete: %s (%s) (%v) ===\n", e.BeadTitle, e.BeadID, e.Duration)
	case *events.BeadFailedEvent:
		fmt.Fprintf(c.output, "=== Bead failed: %s (%s) — %s ===\n", e.BeadTitle, e.BeadID, e.Error)
	case *events.BeadStuckEvent:
		fmt.Fprintf(c.output, "=== Bead stuck: %s (%s) — %s ===\n", e.BeadTitle, e.BeadID, e.Reason)
	case *events.BeadSkippedEvent:
		fmt.Fprintf(c.output, "=== Bead skipped: %s — %s ===\n", e.BeadID, e.Reason)
	case *events.BuildStartEvent:
		fmt.Fprintf(c.output, "    Building (%s, attempt %d/%d)...\n", e.Model, e.Attempt, e.MaxAttempts)
	case *events.BuildCompleteEvent:
		status := "SUCCESS"
		if !e.Success {
			status = "FAILED"
		}
		fmt.Fprintf(c.output, "    Build %s: %.4f USD, %d in, %d out, %v\n", status, e.Cost, e.TokensIn, e.TokensOut, e.Duration)
	case *events.ValidationStartEvent:
		fmt.Fprintf(c.output, "    Validating... (%d commands)\n", len(e.Commands))
	case *events.ValidationPassEvent:
		fmt.Fprintf(c.output, "    Validation PASS: %v\n", e.Duration)
	case *events.ValidationFailEvent:
		fmt.Fprintf(c.output, "    Validation FAIL: %v\n", e.Duration)
		if e.Output != "" {
			fmt.Fprintf(c.output, "%s\n", e.Output)
		}
	case *events.ReviewStartEvent:
		fmt.Fprintf(c.output, "    Review (%s%s)...\n", e.Model, thoroughSuffix(e.Thorough))
	case *events.ReviewCompleteEvent:
		fmt.Fprintf(c.output, "    Review %s\n", e.Verdict)
	case *events.AnalysisStartEvent:
		fmt.Fprintf(c.output, "    Analyzing failure...\n")
	case *events.AnalysisCompleteEvent:
		fmt.Fprintf(c.output, "    Analysis: %s (recoverable: %v)\n", e.Category, e.Recoverable)
	case *events.RetroStartEvent:
		fmt.Fprintf(c.output, "    Retrospective...\n")
	case *events.RetroCompleteEvent:
		fmt.Fprintf(c.output, "    Retrospective complete: %d learnings, rules updated: %v\n", e.ProvisionalLearnings, e.RulesUpdated)
	case *events.HeartbeatEvent:
		// Heartbeat events use carriage return overwrite for in-place updates
		heartbeatMsg := fmt.Sprintf("      [%.1fs] %d tools, %d files, %d rate limits\r", e.Elapsed.Seconds(), e.ToolCalls, e.FilesModified, e.RateLimitHits)
		c.output.WriteOverwrite([]byte(heartbeatMsg))
	case *events.ModelSelectedEvent:
		fmt.Fprintf(c.output, "    Model: %s (%s)\n", e.Model, e.Reason)
	case *events.EscalationEvent:
		fmt.Fprintf(c.output, "    Escalation: %s → %s (attempt %d) — %s\n", e.FromModel, e.ToModel, e.Attempt, e.Reason)
	case *events.StallDetectedEvent:
		fmt.Fprintf(c.output, "    Stall detected: %.1fs (threshold: %.1fs)\n", e.Elapsed.Seconds(), e.Threshold.Seconds())
	case *events.ScopeCheckEvent:
		status := "REJECTED"
		if e.Approved {
			status = "APPROVED"
		}
		fmt.Fprintf(c.output, "    Scope: %s (%s) — %s\n", e.Complexity, status, e.Reason)
	case *events.DecomposeStartEvent:
		fmt.Fprintf(c.output, "    Decomposing: %s (%s)...\n", e.BeadTitle, e.BeadID)
	case *events.SubBeadCreatedEvent:
		fmt.Fprintf(c.output, "      Sub-bead %d/%d: %s (%s)\n", e.Index, e.Total, e.SubBeadTitle, e.SubBeadID)
	case *events.DecomposeCompleteEvent:
		fmt.Fprintf(c.output, "    Decomposition complete: %d sub-beads created\n", e.SubBeadsCreated)
	case *events.LogEvent:
		// Transitional: print log events as-is
		fmt.Fprintf(c.output, "[%s] %s\n", e.Level, e.Message)
	default:
		// Unknown event type, silently ignore
	}
}

func thoroughSuffix(thorough bool) string {
	if thorough {
		return ", thorough"
	}
	return ""
}
