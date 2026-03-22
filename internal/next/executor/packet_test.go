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

func TestCompileTaskPacket_IncludesDoctrineWhenProvided(t *testing.T) {
	pkt, err := CompileTaskPacket(TaskPacketInput{
		SpecPacket: "build feature X",
		TaskID:     "t-001",
		Objective:  "implement parser",
		Doctrine:   "Always write tests first. Keep functions small.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pkt, "## Doctrine") {
		t.Fatal("packet must contain Doctrine header")
	}
	if !strings.Contains(pkt, "Always write tests first. Keep functions small.") {
		t.Fatal("packet must contain doctrine content")
	}
}

func TestCompileTaskPacket_OmitsDoctrineWhenEmpty(t *testing.T) {
	pkt, err := CompileTaskPacket(TaskPacketInput{
		SpecPacket: "build feature X",
		TaskID:     "t-001",
		Objective:  "implement parser",
		Doctrine:   "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pkt, "Doctrine") {
		t.Fatal("packet must not contain Doctrine section when empty")
	}
}

func TestCompileTaskPacket_IncludesRefinementGuidanceWhenProvided(t *testing.T) {
	pkt, err := CompileTaskPacket(TaskPacketInput{
		SpecPacket:         "build feature X",
		TaskID:             "t-001",
		Objective:          "implement parser",
		RefinementGuidance: "Focus on edge cases. Validate input thoroughly.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pkt, "## Refinement Guidance") {
		t.Fatal("packet must contain Refinement Guidance header")
	}
	if !strings.Contains(pkt, "Focus on edge cases. Validate input thoroughly.") {
		t.Fatal("packet must contain refinement guidance content")
	}
}

func TestCompileTaskPacket_OmitsRefinementGuidanceWhenEmpty(t *testing.T) {
	pkt, err := CompileTaskPacket(TaskPacketInput{
		SpecPacket:         "build feature X",
		TaskID:             "t-001",
		Objective:          "implement parser",
		RefinementGuidance: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pkt, "Refinement Guidance") {
		t.Fatal("packet must not contain Refinement Guidance section when empty")
	}
}
