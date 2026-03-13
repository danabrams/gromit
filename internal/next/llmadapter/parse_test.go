package llmadapter

import "testing"

func TestExtractJSON_BareJSON(t *testing.T) {
	input := `[{"key":"value"}]`
	got := ExtractJSON(input)
	if got != input {
		t.Errorf("expected %q, got %q", input, got)
	}
}

func TestExtractJSON_MarkdownFencedJSON(t *testing.T) {
	input := "Here is the result:\n```json\n{\"key\":\"value\"}\n```\nDone."
	want := `{"key":"value"}`
	got := ExtractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_NestedFences(t *testing.T) {
	input := "```json\n{\"a\":1}\n```\nsome text\n```json\n{\"b\":2}\n```"
	want := `{"a":1}`
	got := ExtractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_NoClosingFence(t *testing.T) {
	input := "```json\n{\"key\":\"value\"}"
	want := `{"key":"value"}`
	got := ExtractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_MultipleFencedBlocks_ReturnsFirst(t *testing.T) {
	input := "```\nfirst block\n```\n```\nsecond block\n```"
	want := "first block"
	got := ExtractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_NoJSONAtAll(t *testing.T) {
	input := "   just some plain text   "
	want := "just some plain text"
	got := ExtractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_RawJSONWithoutFences(t *testing.T) {
	input := `  {"status":"pass","rationale":"ok"}  `
	want := `{"status":"pass","rationale":"ok"}`
	got := ExtractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_ProsePrefixedJSON(t *testing.T) {
	input := `Here is the result: {"status":"pass"}`
	want := `{"status":"pass"}`
	got := ExtractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
