package execution

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
)

// --- OverwriteWriter mock ---

// mockOverwriteWriter captures both normal and overwrite writes for assertion.
type mockOverwriteWriter struct {
	mu             sync.Mutex
	normalWrites   []string
	overwriteCalls []string
}

func (m *mockOverwriteWriter) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.normalWrites = append(m.normalWrites, string(p))
	return len(p), nil
}

func (m *mockOverwriteWriter) WriteOverwrite(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overwriteCalls = append(m.overwriteCalls, string(p))
	return len(p), nil
}

// --- Heartbeat tests ---

// Expected failure: StartHeartbeat function does not exist in execution/ package yet
func TestStartHeartbeat_StallDetectionFiresOnStall(t *testing.T) {
	// Tests that the heartbeat goroutine detects a stall when no events arrive
	// within the stall timeout, and calls the onStall callback.
	stats, err := logger.NewStreamStats()
	if err != nil {
		t.Fatalf("failed to create StreamStats: %v", err)
	}
	// Simulate an event so HasReceivedEvent returns true (stall detection only
	// activates after the first event)
	stats.RecordEvent()

	stallFired := make(chan struct{}, 1)
	onStall := func() {
		select {
		case stallFired <- struct{}{}:
		default:
		}
	}

	cfg := HeartbeatConfig{
		InitialDelay:   10 * time.Millisecond,
		HeartbeatRate:  100 * time.Millisecond,
		StallCheckRate: 20 * time.Millisecond,
	}

	out := &mockOverwriteWriter{}

	stop := StartHeartbeatWithConfig(stats, 50*time.Millisecond, 0, onStall, cfg, nil, out)
	defer stop()

	// Wait for stall detection to fire (stall timeout is 50ms, check rate is 20ms)
	select {
	case <-stallFired:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("stall detection did not fire within timeout")
	}
}

// Expected failure: StartHeartbeat function does not exist in execution/ package yet
func TestStartHeartbeat_TwoTierStallTimeout(t *testing.T) {
	// Tests the two-tier stall detection: before tool activity, uses stallTimeout;
	// after tool activity, uses stallTimeoutActive (longer).
	stats, err := logger.NewStreamStats()
	if err != nil {
		t.Fatalf("failed to create StreamStats: %v", err)
	}
	stats.RecordEvent()
	// Simulate tool activity so HasToolActivity returns true
	stats.RecordToolCall("Edit", "/tmp/file.go")

	stallTierCaptured := ""
	stallFired := make(chan struct{}, 1)
	onStall := func() {
		select {
		case stallFired <- struct{}{}:
		default:
		}
	}

	cfg := HeartbeatConfig{
		InitialDelay:   5 * time.Millisecond,
		HeartbeatRate:  500 * time.Millisecond,
		StallCheckRate: 10 * time.Millisecond,
	}

	out := &mockOverwriteWriter{}

	// stallTimeout = 20ms (initial, unused since tool activity exists)
	// stallTimeoutActive = 40ms (active, used since tool activity exists)
	stop := StartHeartbeatWithConfig(stats, 20*time.Millisecond, 40*time.Millisecond, onStall, cfg, nil, out)
	defer stop()

	select {
	case <-stallFired:
		// Verify that the stall was recorded with "active" tier (since tool activity existed)
		_, stallTierCaptured, _, _, _, _ = stats.DiagnosticSnapshot()
		if stallTierCaptured != "active" {
			t.Errorf("stall tier = %q, want %q (tool activity was present)", stallTierCaptured, "active")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stall detection did not fire within timeout")
	}
}

// Expected failure: StartHeartbeat function does not exist in execution/ package yet
func TestStartHeartbeat_ToolCallEventsUpdateDisplay(t *testing.T) {
	// When tool call events arrive on the channel, the heartbeat should update
	// the display in-place using OverwriteWriter.
	stats, err := logger.NewStreamStats()
	if err != nil {
		t.Fatalf("failed to create StreamStats: %v", err)
	}

	cfg := HeartbeatConfig{
		InitialDelay:   5 * time.Millisecond,
		HeartbeatRate:  10 * time.Second, // long interval to avoid periodic prints
		StallCheckRate: 0,                // disable stall detection
	}

	out := &mockOverwriteWriter{}
	toolCallEvents := make(chan claude.ToolEvent, 10)

	stop := StartHeartbeatWithConfig(stats, 0, 0, nil, cfg, toolCallEvents, out)

	// Wait for initial heartbeat
	time.Sleep(20 * time.Millisecond)

	// Send tool call events
	for i := 0; i < 3; i++ {
		stats.RecordToolCall("Edit", "/tmp/file.go")
		toolCallEvents <- claude.ToolEvent{
			ToolName:  "Edit",
			FilePath:  "/tmp/file.go",
			Timestamp: time.Now(),
		}
		time.Sleep(5 * time.Millisecond)
	}

	stop()

	out.mu.Lock()
	defer out.mu.Unlock()

	if len(out.overwriteCalls) == 0 {
		t.Fatal("expected at least one overwrite call from tool call events")
	}

	// Overwrite calls should contain carriage return for in-place update
	for i, call := range out.overwriteCalls {
		if !strings.HasPrefix(call, "\r") {
			t.Errorf("overwrite call %d does not start with \\r: %q", i, call)
		}
	}
}

// Expected failure: StartHeartbeat function does not exist in execution/ package yet
func TestStartHeartbeat_StopFunctionCleansUpGoroutine(t *testing.T) {
	// The stop function returned by StartHeartbeat should cleanly shut down
	// the goroutine without blocking indefinitely.
	stats, err := logger.NewStreamStats()
	if err != nil {
		t.Fatalf("failed to create StreamStats: %v", err)
	}

	cfg := HeartbeatConfig{
		InitialDelay:   10 * time.Millisecond,
		HeartbeatRate:  50 * time.Millisecond,
		StallCheckRate: 0,
	}

	out := &mockOverwriteWriter{}
	stop := StartHeartbeatWithConfig(stats, 0, 0, nil, cfg, nil, out)

	// Wait long enough for initial heartbeat to fire
	time.Sleep(30 * time.Millisecond)

	// Stop should return promptly
	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()

	select {
	case <-done:
		// expected - stop completed
	case <-time.After(2 * time.Second):
		t.Fatal("stop function did not return within timeout")
	}
}

// Expected failure: PrintHeartbeat function does not exist in execution/ package yet
func TestPrintHeartbeat_FormatsStatusLine(t *testing.T) {
	// PrintHeartbeat should format "[Xm XXs] Y tool calls, Z files modified"
	// from StreamStats data.
	stats, err := logger.NewStreamStats()
	if err != nil {
		t.Fatalf("failed to create StreamStats: %v", err)
	}

	// Record some tool calls to get non-zero values
	stats.RecordToolCall("Edit", "/tmp/a.go")
	stats.RecordToolCall("Read", "")
	stats.RecordToolCall("Write", "/tmp/b.go")

	var buf bytes.Buffer
	line := PrintHeartbeat(stats, &buf)

	if !strings.Contains(line, "3 tool calls") {
		t.Errorf("heartbeat line should contain '3 tool calls', got: %q", line)
	}
	if !strings.Contains(line, "2 files modified") {
		t.Errorf("heartbeat line should contain '2 files modified', got: %q", line)
	}
	// Should have time format [Xm XXs]
	if !strings.HasPrefix(line, "[") {
		t.Errorf("heartbeat line should start with '[', got: %q", line)
	}

	// Should also be written to the output
	if buf.Len() == 0 {
		t.Error("PrintHeartbeat should write to the output writer")
	}
}

// Expected failure: PrintHeartbeat function does not exist in execution/ package yet
func TestPrintHeartbeat_WaitingMessageWhenNoToolCalls(t *testing.T) {
	// When no tool calls have been made, PrintHeartbeat should show a waiting message.
	stats, err := logger.NewStreamStats()
	if err != nil {
		t.Fatalf("failed to create StreamStats: %v", err)
	}

	var buf bytes.Buffer
	line := PrintHeartbeat(stats, &buf)

	if !strings.Contains(line, "Waiting") {
		t.Errorf("heartbeat with 0 tool calls should contain 'Waiting', got: %q", line)
	}
}

// Expected failure: PrintHeartbeat function does not exist in execution/ package yet
func TestPrintHeartbeat_NilSafetyReturnsEmpty(t *testing.T) {
	// PrintHeartbeat with nil stats should return empty string without panicking.
	var buf bytes.Buffer
	line := PrintHeartbeat(nil, &buf)

	if line != "" {
		t.Errorf("PrintHeartbeat(nil) should return empty string, got: %q", line)
	}
}

// Expected failure: HeartbeatConfig type does not exist in execution/ package yet
func TestHeartbeatConfig_IsExported(t *testing.T) {
	// HeartbeatConfig must be an exported type so callers can customize timing.
	cfg := HeartbeatConfig{
		InitialDelay:   15 * time.Second,
		HeartbeatRate:  30 * time.Second,
		StallCheckRate: 10 * time.Second,
	}

	if cfg.InitialDelay != 15*time.Second {
		t.Errorf("InitialDelay = %v, want 15s", cfg.InitialDelay)
	}
	if cfg.HeartbeatRate != 30*time.Second {
		t.Errorf("HeartbeatRate = %v, want 30s", cfg.HeartbeatRate)
	}
	if cfg.StallCheckRate != 10*time.Second {
		t.Errorf("StallCheckRate = %v, want 10s", cfg.StallCheckRate)
	}
}

// Expected failure: OverwriteWriter interface does not exist in execution/ package yet
func TestOverwriteWriter_InterfaceDefinedNarrowly(t *testing.T) {
	// The OverwriteWriter interface should be defined with Write and WriteOverwrite
	// methods. This test verifies mockOverwriteWriter satisfies it.
	var _ OverwriteWriter = (*mockOverwriteWriter)(nil)
}

// Expected failure: StartHeartbeat wrapper function does not exist in execution/ package yet
func TestStartHeartbeat_DefaultConfigUsedByWrapper(t *testing.T) {
	// StartHeartbeat (the convenience wrapper) should use DefaultHeartbeatConfig
	// internally. We verify by calling it and seeing that it functions correctly.
	stats, err := logger.NewStreamStats()
	if err != nil {
		t.Fatalf("failed to create StreamStats: %v", err)
	}

	out := &mockOverwriteWriter{}
	stop := StartHeartbeat(stats, 0, 0, nil, nil, out)
	// Should be callable and return a stop function
	stop()
}
