package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestFakeAgent_RecordsCallsAndReturnsConfiguredResponse(t *testing.T) {
	agent := &FakeAgent{
		Response: "mock response",
	}

	resp, err := agent.Run(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp != "mock response" {
		t.Errorf("response = %q, want %q", resp, "mock response")
	}
	if len(agent.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(agent.Calls))
	}
	if agent.Calls[0] != "test prompt" {
		t.Errorf("call[0] = %q, want %q", agent.Calls[0], "test prompt")
	}
}

func TestFakeAgent_ReturnsConfiguredError(t *testing.T) {
	agent := &FakeAgent{
		Err: fmt.Errorf("agent failure"),
	}

	_, err := agent.Run(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFakeGit_DiffReturnsConfiguredOutput(t *testing.T) {
	g := &FakeGit{
		DiffOutput: "diff --git a/file.go b/file.go",
	}

	diff, err := g.Diff("main")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff != g.DiffOutput {
		t.Errorf("diff = %q, want %q", diff, g.DiffOutput)
	}
}

func TestFakeClock_ReturnsConfiguredTime(t *testing.T) {
	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	c := &FakeClock{NowTime: now}

	if got := c.Now(); got != now {
		t.Errorf("Now() = %v, want %v", got, now)
	}
}

func TestFakeClock_Advance(t *testing.T) {
	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	c := &FakeClock{NowTime: now}
	c.Advance(5 * time.Minute)

	want := now.Add(5 * time.Minute)
	if got := c.Now(); got != want {
		t.Errorf("Now() after Advance = %v, want %v", got, want)
	}
}

func TestFakeCmdRunner_ReturnsConfiguredOutput(t *testing.T) {
	r := &FakeCmdRunner{
		Outputs: map[string]CmdResult{
			"go test ./...": {Stdout: "PASS", ExitCode: 0},
		},
	}

	result := r.Run("go test ./...")
	if result.Stdout != "PASS" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "PASS")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestFakeCmdRunner_UnknownCommandReturnsError(t *testing.T) {
	r := &FakeCmdRunner{
		Outputs: map[string]CmdResult{},
	}

	result := r.Run("unknown command")
	if result.ExitCode == 0 {
		t.Error("unknown command should return non-zero exit code")
	}
}
