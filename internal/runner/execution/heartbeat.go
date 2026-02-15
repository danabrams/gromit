package execution

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
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

// StartHeartbeat launches a heartbeat goroutine with default timing config.
// Returns a function to stop the heartbeat.
func StartHeartbeat(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func(), toolCallEvents <-chan claude.ToolEvent, out OverwriteWriter) func() {
	return StartHeartbeatWithConfig(stats, stallTimeout, stallTimeoutActive, onStall, DefaultHeartbeatConfig, toolCallEvents, out)
}

// StartHeartbeatWithConfig launches a goroutine that prints periodic status updates
// and optionally detects stalls using two-tier timeouts:
//   - stallTimeout: used before Claude has made any tool calls (detecting true hangs)
//   - stallTimeoutActive: used after tool activity (longer, allows thinking pauses)
//
// The toolCallEvents channel (optional, can be nil) feeds real-time tool call notifications.
// Returns a function to stop the heartbeat.
func StartHeartbeatWithConfig(stats *logger.StreamStats, stallTimeout, stallTimeoutActive time.Duration, onStall func(), cfg HeartbeatConfig, toolCallEvents <-chan claude.ToolEvent, out OverwriteWriter) func() {
	if stats == nil {
		return func() {}
	}
	done := make(chan struct{})
	finished := make(chan struct{})
	lastHeartbeatLine := ""

	go func() {
		defer close(finished)

		// First heartbeat sooner so hangs are visible quickly
		select {
		case <-done:
			return
		case <-time.After(cfg.InitialDelay):
		}
		lastHeartbeatLine = PrintHeartbeat(stats, out)

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
					lastHeartbeatLine = PrintHeartbeat(stats, out)
				case <-stallTicker.C:
					if stats.HasReceivedEvent() {
						threshold := stallTimeout
						tier := "initial"
						if stats.HasToolActivity() && stallTimeoutActive > 0 {
							threshold = stallTimeoutActive
							tier = "active"
						}
						if stats.TimeSinceLastEvent() >= threshold {
							stats.RecordStall(tier)
							onStall()
							return
						}
					}
				case event := <-toolCallEvents:
					_ = event
					lastHeartbeatLine = overwriteHeartbeat(stats, lastHeartbeatLine, out)
				}
			} else {
				select {
				case <-done:
					return
				case <-heartbeatTicker.C:
					lastHeartbeatLine = PrintHeartbeat(stats, out)
				case event := <-toolCallEvents:
					_ = event
					lastHeartbeatLine = overwriteHeartbeat(stats, lastHeartbeatLine, out)
				}
			}
		}
	}()

	return func() {
		close(done)
		<-finished
	}
}

// PrintHeartbeat formats and writes a status line from StreamStats data.
// Returns the formatted line string.
func PrintHeartbeat(stats *logger.StreamStats, output io.Writer) string {
	if stats == nil {
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
	_, _ = fmt.Fprintf(output, "%s\n", line) // best-effort output, explicitly discard error
	return line
}

// overwriteHeartbeat updates the heartbeat line in-place using carriage return.
func overwriteHeartbeat(stats *logger.StreamStats, lastLine string, out OverwriteWriter) string {
	if stats == nil || out == nil {
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

	padding := ""
	if len(lastLine) > len(newLine) {
		padding = strings.Repeat(" ", len(lastLine)-len(newLine))
	}
	if _, err := out.WriteOverwrite([]byte(fmt.Sprintf("\r%s%s", newLine, padding))); err != nil {
		return lastLine
	}

	return newLine
}
