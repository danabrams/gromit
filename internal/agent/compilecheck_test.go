package agent

import (
	"os"
	"strings"
	"testing"
)

func TestAgentDefinesCompileTimeCheck(t *testing.T) {
	data, err := os.ReadFile("agent.go")
	if err != nil {
		t.Fatalf("read agent.go: %v", err)
	}

	if !strings.Contains(string(data), "var _ Agent = (*cliAgent)(nil)") {
		t.Fatalf("agent.go must define a compile-time check that cliAgent implements Agent")
	}
}
