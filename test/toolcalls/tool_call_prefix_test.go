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

func TestToolCallPrefixesContainsExpectedEntries(t *testing.T) {
	prefixes := ToolCallPrefixes()

	if len(prefixes) != 3 {
		t.Fatalf("expected 3 entries in ToolCallPrefixes, got %d", len(prefixes))
	}

	cases := map[ToolCallKind]string{
		ToolCallCodex:  "codex",
		ToolCallClaude: "claude",
		ToolCallBD:     "bd",
	}

	for kind, expected := range cases {
		prefix, ok := prefixes[kind]
		if !ok {
			t.Fatalf("ToolCallPrefixes does not contain %q", kind)
		}
		if prefix != expected {
			t.Fatalf("ToolCallPrefixes[%q] = %q, want %q", kind, prefix, expected)
		}
	}
}

func TestToolCallPrefixesReturnsCopy(t *testing.T) {
	prefixes := ToolCallPrefixes()
	prefixes[ToolCallCodex] = "mutated"

	fresh := ToolCallPrefixes()
	if fresh[ToolCallCodex] != "codex" {
		t.Fatalf("expected fresh copy to preserve prefix, got %q", fresh[ToolCallCodex])
	}
}
