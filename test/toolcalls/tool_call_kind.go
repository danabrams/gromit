package toolcalls

import "fmt"

type ToolCallKind string

const (
	ToolCallCodex  ToolCallKind = "codex"
	ToolCallClaude ToolCallKind = "claude"
	ToolCallBD     ToolCallKind = "bd"
)

var ToolCallPrefixMap = map[ToolCallKind]string{
	ToolCallCodex:  "codex",
	ToolCallClaude: "claude",
	ToolCallBD:     "bd",
}

// toolCallPrefixes is kept for backward compatibility with ToolCallPrefix function
var toolCallPrefixes = ToolCallPrefixMap

func ToolCallPrefix(kind ToolCallKind) (string, error) {
	prefix, ok := toolCallPrefixes[kind]
	if !ok {
		return "", fmt.Errorf("unknown tool call kind: %q", kind)
	}
	return prefix, nil
}
