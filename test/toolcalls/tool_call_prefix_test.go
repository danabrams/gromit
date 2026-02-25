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

func TestToolCallPrefixMapExported(t *testing.T) {
	if ToolCallPrefixMap == nil {
		t.Fatal("ToolCallPrefixMap is nil")
	}

	if len(ToolCallPrefixMap) != 3 {
		t.Fatalf("expected 3 entries in ToolCallPrefixMap, got %d", len(ToolCallPrefixMap))
	}

	cases := map[ToolCallKind]string{
		ToolCallCodex:  "codex",
		ToolCallClaude: "claude",
		ToolCallBD:     "bd",
	}

	for kind, expected := range cases {
		prefix, ok := ToolCallPrefixMap[kind]
		if !ok {
			t.Fatalf("ToolCallPrefixMap does not contain %q", kind)
		}
		if prefix != expected {
			t.Fatalf("ToolCallPrefixMap[%q] = %q, want %q", kind, prefix, expected)
		}
	}
}
