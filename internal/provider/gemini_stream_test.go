package provider

import (
	"strings"
	"testing"
)

func TestParseGeminiStream_AssistantText(t *testing.T) {
	t.Parallel()

	data := strings.Join([]string{
		`{"type":"message","role":"assistant","content":"Hello"}`,
		`{"type":"message","role":"assistant","content":" world"}`,
	}, "\n")

	result, err := parseGeminiStream(strings.NewReader(data))
	if err != nil {
		t.Fatalf("parseGeminiStream failed: %v", err)
	}

	if want := "Hello world"; result.AssistantText != want {
		t.Fatalf("assistant text = %q, want %q", result.AssistantText, want)
	}
}
