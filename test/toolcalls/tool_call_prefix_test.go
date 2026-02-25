package toolcalls

import "testing"

func TestToolCallPrefixMapping(t *testing.T) {
	cases := map[ToolCallKind]string{
		ToolCallCodex:  "codex",
		ToolCallClaude: "claude",
		ToolCallBD:     "bd",
	}

	for kind, expected := range cases {
		prefix, err := ToolCallPrefix(kind)
		if err != nil {
			t.Fatalf("ToolCallPrefix(%q) returned error: %v", kind, err)
		}
		if prefix != expected {
			t.Fatalf("ToolCallPrefix(%q) = %q, want %q", kind, prefix, expected)
		}
	}

	if _, err := ToolCallPrefix(ToolCallKind("unknown")); err == nil {
		t.Fatal("expected error for unknown ToolCallKind")
	}
}
