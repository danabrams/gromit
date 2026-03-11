package executor

import (
	"strings"
	"testing"
)

func TestCompileTaskPacket(t *testing.T) {
	pkt, err := CompileTaskPacket(TaskPacketInput{
		SpecPacket:   "build feature X",
		TaskID:       "t-001",
		Objective:    "implement parser",
		ProofChecks:  []string{"go test ./parser/"},
		ExpectedArea: []string{"internal/parser/"},
		PriorContext: "tasks t-000 completed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pkt == "" {
		t.Fatal("packet must not be empty")
	}
	if !strings.Contains(pkt, "implement parser") {
		t.Fatal("packet must contain objective")
	}
	if !strings.Contains(pkt, "go test ./parser/") {
		t.Fatal("packet must contain proof checks")
	}
}
