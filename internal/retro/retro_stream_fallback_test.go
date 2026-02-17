package retro

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

type streamFallbackProvider struct {
	streamResult *provider.Result
	streamErr    error
	runResult    *provider.Result
	runErr       error
	streamCalls  int
	runCalls     int
}

func (m *streamFallbackProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	m.runCalls++
	return m.runResult, m.runErr
}

func (m *streamFallbackProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	m.streamCalls++
	return m.streamResult, m.streamErr
}

func TestRunAnalysis_FallsBackToRunWhenStreamOutputEmpty(t *testing.T) {
	p := &streamFallbackProvider{
		streamResult: &provider.Result{Success: true, Output: "   "},
		runResult:    &provider.Result{Success: true, Output: "fallback analysis"},
	}

	r := &Retro{provider: p}
	got, err := r.runAnalysis(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("runAnalysis() error = %v", err)
	}
	if got == nil {
		t.Fatal("runAnalysis() returned nil result")
	}
	if got.GetOutput() != "fallback analysis" {
		t.Fatalf("runAnalysis() output = %q, want fallback output", got.GetOutput())
	}
	if p.streamCalls != 1 {
		t.Fatalf("streamCalls = %d, want 1", p.streamCalls)
	}
	if p.runCalls != 1 {
		t.Fatalf("runCalls = %d, want 1", p.runCalls)
	}
}

func TestRunAnalysis_KeepsStreamOutputWhenPresent(t *testing.T) {
	p := &streamFallbackProvider{
		streamResult: &provider.Result{Success: true, Output: "stream analysis"},
		runResult:    &provider.Result{Success: true, Output: "fallback analysis"},
	}

	r := &Retro{provider: p}
	got, err := r.runAnalysis(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("runAnalysis() error = %v", err)
	}
	if got == nil {
		t.Fatal("runAnalysis() returned nil result")
	}
	if got.GetOutput() != "stream analysis" {
		t.Fatalf("runAnalysis() output = %q, want stream output", got.GetOutput())
	}
	if p.streamCalls != 1 {
		t.Fatalf("streamCalls = %d, want 1", p.streamCalls)
	}
	if p.runCalls != 0 {
		t.Fatalf("runCalls = %d, want 0", p.runCalls)
	}
}

func TestRunAnalysis_ReturnsErrorWhenBothOutputsEmpty(t *testing.T) {
	p := &streamFallbackProvider{
		streamResult: &provider.Result{Success: true, Output: ""},
		runResult:    &provider.Result{Success: true, Output: "   "},
	}

	r := &Retro{provider: p}
	got, err := r.runAnalysis(context.Background(), "prompt")
	if err == nil {
		t.Fatal("runAnalysis() error = nil, want empty output error")
	}
	if got != nil {
		t.Fatal("runAnalysis() result should be nil on empty output")
	}
}
