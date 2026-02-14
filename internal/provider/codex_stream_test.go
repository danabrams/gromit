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
