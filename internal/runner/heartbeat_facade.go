package runner

import (
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
)

var _ = claude.ToolEvent{}

// heartbeatInterval is the interval at which Claude's progress is checked and heartbeat status is printed.
const heartbeatInterval = 30 * time.Second

// heartbeatConfig holds timing parameters for the heartbeat goroutine.
type heartbeatConfig struct {
	InitialDelay   time.Duration
	HeartbeatRate  time.Duration
	StallCheckRate time.Duration
}

var defaultHeartbeatConfig = heartbeatConfig{
	InitialDelay:   15 * time.Second,
	HeartbeatRate:  heartbeatInterval,
	StallCheckRate: 10 * time.Second,
}

var _ = (*Runner).startHeartbeat

// startHeartbeat launches a goroutine that prints periodic status updates and listens
// for tool call events to update the display in real-time. It also optionally detects
// stalls using two-tier timeouts:
//   - stallTimeout: used before Claude has made any tool calls (detecting true hangs)
//   - stallTimeoutActive: used after tool activity (longer, allows thinking pauses)
//
// The toolCallEvents channel (optional, can be nil) feeds real-time tool call notifications.
// Returns a function to stop the heartbeat.
func (r *Runner) startHeartbeat(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func(), toolCallEvents <-chan provider.ToolEvent) func() {
	return r.startHeartbeatWithConfig(stats, stallTimeout, stallTimeoutActive, onStall, defaultHeartbeatConfig, toolCallEvents)
}

func (r *Runner) startHeartbeatWithConfig(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func(), cfg heartbeatConfig, toolCallEvents <-chan provider.ToolEvent) func() {
	if r == nil || stats == nil {
		return func() {}
	}
	done := make(chan struct{})
	usedOverwrite := make(chan bool)
	lastHeartbeatLine := "" // Track last printed line for overwriting
	go func() {
		// First heartbeat sooner so hangs are visible quickly
		select {
		case <-done:
			usedOverwrite <- false
			return
		case <-time.After(cfg.InitialDelay):
		}
		lastHeartbeatLine = r.printHeartbeat(stats)

		heartbeatTicker := time.NewTicker(cfg.HeartbeatRate)
		defer heartbeatTicker.Stop()

		// Stall check runs at shorter intervals for reasonable precision
		var stallTicker *time.Ticker
		if stallTimeout > 0 && onStall != nil {
			stallTicker = time.NewTicker(cfg.StallCheckRate)
			defer stallTicker.Stop()
		}

		wasOverwritten := false
		for {
			if stallTicker != nil {
				select {
				case <-done:
					usedOverwrite <- wasOverwritten
					return
				case <-heartbeatTicker.C:
					lastHeartbeatLine = r.printHeartbeat(stats)
					wasOverwritten = false
				case <-stallTicker.C:
					// Only check for stalls after the first stream event arrives.
					// Before that, Claude CLI is still starting up — the startup
					// monitor in claude.go handles that phase separately.
					if stats.HasReceivedEvent() {
						// Two-tier timeout: use longer timeout once Claude has
						// started making tool calls (reading files, editing, etc.)
						threshold := stallTimeout
						tier := "initial"
						if stats.HasToolActivity() && stallTimeoutActive > 0 {
							threshold = stallTimeoutActive
							tier = "active"
						}
						if stats.TimeSinceLastEvent() >= threshold {
							stats.RecordStall(tier)
							r.log("STALL DETECTED (%s): No output from Claude for %v (threshold: %v)",
								tier, stats.TimeSinceLastEvent().Round(time.Second), threshold)
							onStall()
							usedOverwrite <- wasOverwritten
							return
						}
					}
				case <-toolCallEvents:
					// On tool call event, update heartbeat line in place
					lastHeartbeatLine = r.overwriteHeartbeat(stats, lastHeartbeatLine)
					wasOverwritten = true
				}
			} else {
				select {
				case <-done:
					usedOverwrite <- wasOverwritten
					return
				case <-heartbeatTicker.C:
					lastHeartbeatLine = r.printHeartbeat(stats)
					wasOverwritten = false
				case <-toolCallEvents:
					// On tool call event, update heartbeat line in place
					lastHeartbeatLine = r.overwriteHeartbeat(stats, lastHeartbeatLine)
					wasOverwritten = true
				}
			}
		}
	}()
	return func() {
		close(done)
		// Wait for the goroutine to signal completion
		// syncWriter handles newline transition automatically
		<-usedOverwrite
	}
}

func (r *Runner) printHeartbeat(stats *logger.StreamStats) string {
	if r == nil || stats == nil {
		return ""
	}
	toolCalls, filesModified, elapsed := stats.Snapshot()
	minutes := int(elapsed.Minutes())
	seconds := int(elapsed.Seconds()) % 60
	var line string
	if toolCalls == 0 {
		line = fmt.Sprintf("[%dm%02ds] Waiting for Claude to respond (may be thinking)...", minutes, seconds)
	} else {
		line = fmt.Sprintf("[%dm%02ds] %d tool calls, %d files modified", minutes, seconds, toolCalls, filesModified)
	}
	r.log("%s", line)
	return line
}

// overwriteHeartbeat updates the heartbeat line in place using carriage return and padding.
// lastLine is the previously printed line (for padding calculation).
// Returns the new line that was printed.
func (r *Runner) overwriteHeartbeat(stats *logger.StreamStats, lastLine string) string {
	if r == nil || r.syncOut == nil || stats == nil {
		return ""
	}
	toolCalls, filesModified, elapsed := stats.Snapshot()
	minutes := int(elapsed.Minutes())
	seconds := int(elapsed.Seconds()) % 60
	var newLine string
	if toolCalls == 0 {
		newLine = fmt.Sprintf("[%dm%02ds] Waiting for Claude to respond (may be thinking)...", minutes, seconds)
	} else {
		newLine = fmt.Sprintf("[%dm%02ds] %d tool calls, %d files modified", minutes, seconds, toolCalls, filesModified)
	}

	// Use carriage return to overwrite the line, pad to clear old content
	padding := ""
	if len(lastLine) > len(newLine) {
		padding = strings.Repeat(" ", len(lastLine)-len(newLine))
	}
	if _, err := r.syncOut.WriteOverwrite([]byte(fmt.Sprintf("\r%s%s", newLine, padding))); err != nil {
		r.log("Warning: failed to overwrite heartbeat: %v", err)
	}

	return newLine
}
