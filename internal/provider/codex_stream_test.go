package provider

import (
	"bytes"
	"strings"
	"testing"
)

// TestProcessCodexStreamEmptyInput verifies that processCodexStream handles empty input.
// Red: processCodexStream function does not exist yet
func TestProcessCodexStreamEmptyInput(t *testing.T) {
	reader := strings.NewReader("")
	var output bytes.Buffer

	resultText, usage, err := processCodexStream(reader, &output, nil, nil)

	if err != nil {
		t.Fatalf("processCodexStream() error = %v, want nil", err)
	}

	if resultText != "" {
		t.Errorf("resultText = %q, want empty string", resultText)
	}

	if usage != nil {
		t.Errorf("usage = %v, want nil for empty input", usage)
	}
}

// TestProcessCodexStreamThreadStartedEvent verifies that thread.started events invoke handler.
// Red: processCodexStream does not emit events for thread.started yet
func TestProcessCodexStreamThreadStartedEvent(t *testing.T) {
	input := `{"type":"thread.started"}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer
	var handlerCalled bool

	handler := func(line []byte) {
		handlerCalled = true
	}

	_, _, err := processCodexStream(reader, &output, handler, nil)

	if err != nil {
		t.Fatalf("processCodexStream() error = %v, want nil", err)
	}

	if !handlerCalled {
		t.Error("EventHandler should be called for thread.started event")
	}
}
