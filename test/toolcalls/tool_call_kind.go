package toolcalls

import "fmt"

type ToolCallKind string

const (
	ToolCallCodex  ToolCallKind = "codex"
	ToolCallClaude ToolCallKind = "claude"
	ToolCallBD     ToolCallKind = "bd"
)

var toolCallPrefixes = map[ToolCallKind]string{
	ToolCallCodex:  "codex",
	ToolCallClaude: "claude",
	ToolCallBD:     "bd",
}

func ToolCallPrefix(kind ToolCallKind) (string, error) {
	prefix, ok := toolCallPrefixes[kind]
	if !ok {
		return "", fmt.Errorf("unknown tool call kind: %q", kind)
	}
	return prefix, nil
}

// ToolCallPrefixes returns a copy of the configured tool prefixes.
func ToolCallPrefixes() map[ToolCallKind]string {
	prefixes := make(map[ToolCallKind]string, len(toolCallPrefixes))
	for kind, prefix := range toolCallPrefixes {
		prefixes[kind] = prefix
	}
	return prefixes
}
