package execution

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
)

// HeartbeatConfig holds timing parameters for the heartbeat goroutine.
type HeartbeatConfig struct {
	InitialDelay   time.Duration
	HeartbeatRate  time.Duration
	StallCheckRate time.Duration
}

// DefaultHeartbeatConfig provides sensible defaults for heartbeat timing.
var DefaultHeartbeatConfig = HeartbeatConfig{
	InitialDelay:   15 * time.Second,
	HeartbeatRate:  30 * time.Second,
	StallCheckRate: 10 * time.Second,
}

// HeartbeatEvent represents metrics emitted by the heartbeat goroutine.
type HeartbeatEvent struct {
	Elapsed            time.Duration
	ToolCalls          int
	FilesModified      int
	RateLimitHits      int
	WaitingForResponse bool
}

// StallDetectedEvent represents stall detection metadata emitted when the
// heartbeat cancels the invocation.
type StallDetectedEvent struct {
	Elapsed   time.Duration
	Threshold time.Duration
}

// HeartbeatSubscriber consumes heartbeat and stall events.
type HeartbeatSubscriber interface {
	HandleHeartbeat(event HeartbeatEvent)
	HandleStall(event StallDetectedEvent)
}

// StartHeartbeat launches a heartbeat goroutine with default timing config.
// Returns a function to stop the heartbeat.
func StartHeartbeat(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func(), toolCallEvents <-chan provider.ToolEvent, subscriber HeartbeatSubscriber) func() {
	return StartHeartbeatWithConfig(stats, stallTimeout, stallTimeoutActive, onStall, DefaultHeartbeatConfig, toolCallEvents, subscriber)
}

// StartHeartbeatWithConfig launches a goroutine that emits heartbeat events
// and optionally detects stalls using two-tier timeouts:
//   - stallTimeout: used before Claude has made any tool calls (detecting true hangs)
//   - stallTimeoutActive: used after tool activity (longer, allows thinking pauses)
//
// The toolCallEvents channel (optional, can be nil) feeds real-time tool call notifications.
// Returns a function to stop the heartbeat.
func StartHeartbeatWithConfig(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func(), cfg HeartbeatConfig, toolCallEvents <-chan provider.ToolEvent, subscriber HeartbeatSubscriber) func() {
	if stats == nil {
		return func() {}
	}
	done := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		defer close(finished)

		// First heartbeat sooner so hangs are visible quickly
		select {
		case <-done:
			return
		case <-time.After(cfg.InitialDelay):
		}
		emitHeartbeat(subscriber, stats)

		heartbeatTicker := time.NewTicker(cfg.HeartbeatRate)
		defer heartbeatTicker.Stop()

		var stallTicker *time.Ticker
		if stallTimeout > 0 && onStall != nil {
			stallTicker = time.NewTicker(cfg.StallCheckRate)
			defer stallTicker.Stop()
		}

		for {
			if stallTicker != nil {
				select {
				case <-done:
					return
				case <-heartbeatTicker.C:
					emitHeartbeat(subscriber, stats)
				case <-stallTicker.C:
					if stats.HasReceivedEvent() {
						threshold := stallTimeout
						tier := "initial"
						if stats.HasToolActivity() && stallTimeoutActive > 0 {
							threshold = stallTimeoutActive
							tier = "active"
						}
						elapsed := stats.TimeSinceLastEvent()
						if elapsed >= threshold {
							stats.RecordStall(tier)
							emitStallEvent(subscriber, elapsed, threshold)
							onStall()
							return
						}
					}
				case event := <-toolCallEvents:
					_ = event
					emitHeartbeat(subscriber, stats)
				}
			} else {
				select {
				case <-done:
					return
				case <-heartbeatTicker.C:
					emitHeartbeat(subscriber, stats)
				case event := <-toolCallEvents:
					_ = event
					emitHeartbeat(subscriber, stats)
				}
			}
		}
	}()

	return func() {
		close(done)
		<-finished
	}
}

func emitHeartbeat(subscriber HeartbeatSubscriber, stats *logger.StreamStats) {
	if subscriber == nil || stats == nil {
		return
	}
	toolCalls, filesModified, elapsed := stats.Snapshot()
	_, _, _, _, rateLimitHits, _ := stats.DiagnosticSnapshot()
	subscriber.HandleHeartbeat(HeartbeatEvent{
		Elapsed:            elapsed,
		ToolCalls:          toolCalls,
		FilesModified:      filesModified,
		RateLimitHits:      rateLimitHits,
		WaitingForResponse: toolCalls == 0,
	})
}

func emitStallEvent(subscriber HeartbeatSubscriber, elapsed, threshold time.Duration) {
	if subscriber == nil {
		return
	}
	subscriber.HandleStall(StallDetectedEvent{
		Elapsed:   elapsed,
		Threshold: threshold,
	})
}

type heartbeatRenderer struct {
	out      OverwriteWriter
	lastLine string
}

func newHeartbeatRenderer(out OverwriteWriter) *heartbeatRenderer {
	if out == nil {
		return nil
	}
	return &heartbeatRenderer{out: out}
}

func (h *heartbeatRenderer) HandleHeartbeat(event HeartbeatEvent) {
	if h == nil || h.out == nil {
		return
	}
	line := formatHeartbeatLine(event.ToolCalls, event.FilesModified, event.Elapsed)
	if line == "" {
		return
	}
	if h.lastLine == "" {
		_, _ = h.out.Write([]byte(fmt.Sprintf("%s\n", line)))
	} else {
		padding := ""
		if len(h.lastLine) > len(line) {
			padding = strings.Repeat(" ", len(h.lastLine)-len(line))
		}
		_, _ = h.out.WriteOverwrite([]byte(fmt.Sprintf("\r%s%s", line, padding)))
	}
	h.lastLine = line
}

func (h *heartbeatRenderer) HandleStall(event StallDetectedEvent) {
	_ = event // currently unused
}

// PrintHeartbeat formats and writes a status line from StreamStats data.
// Returns the formatted line string.
func PrintHeartbeat(stats *logger.StreamStats, output io.Writer) string {
	if stats == nil {
		return ""
	}
	toolCalls, filesModified, elapsed := stats.Snapshot()
	line := formatHeartbeatLine(toolCalls, filesModified, elapsed)
	_, _ = fmt.Fprintf(output, "%s\n", line) // best-effort output, explicitly discard error
	return line
}

// overwriteHeartbeat updates the heartbeat line in-place using carriage return.
func overwriteHeartbeat(stats *logger.StreamStats, lastLine string, out OverwriteWriter) string {
	if stats == nil || out == nil {
		return ""
	}
	toolCalls, filesModified, elapsed := stats.Snapshot()
	newLine := formatHeartbeatLine(toolCalls, filesModified, elapsed)

	padding := ""
	if len(lastLine) > len(newLine) {
		padding = strings.Repeat(" ", len(lastLine)-len(newLine))
	}
	if _, err := out.WriteOverwrite([]byte(fmt.Sprintf("\r%s%s", newLine, padding))); err != nil {
		return lastLine
	}

	return newLine
}

func formatHeartbeatLine(toolCalls, filesModified int, elapsed time.Duration) string {
	minutes := int(elapsed.Minutes())
	seconds := int(elapsed.Seconds()) % 60
	if toolCalls == 0 {
		return fmt.Sprintf("[%dm%02ds] Waiting for agent to respond (may be thinking)...", minutes, seconds)
	}
	return fmt.Sprintf("[%dm%02ds] %d tool calls, %d files modified", minutes, seconds, toolCalls, filesModified)
}
