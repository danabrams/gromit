package prompt

import (
	"strings"
	"testing"
)

func TestPromptAssemblerAddsLayerMarkers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "project layer", "instance layer", "fragment layer")
	output := assembler.Assemble()

	expectedSequence := []string{
		"=== BASE ===",
		"base layer",
		"=== PROJECT ===",
		"project layer",
		"=== INSTANCE ===",
		"instance layer",
		"=== FRAGMENT ===",
		"fragment layer",
	}

	lastIndex := 0
	for _, fragment := range expectedSequence {
		idx := strings.Index(output, fragment)
		if idx == -1 {
			t.Fatalf("output missing %q", fragment)
		}
		if idx < lastIndex {
			t.Fatalf("%q appears out of order", fragment)
		}
		lastIndex = idx
	}
}

func TestPromptAssemblerSkipsEmptyLayers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "", "instance layer", "")
	output := assembler.Assemble()

	if !strings.Contains(output, "=== BASE ===") {
		t.Fatalf("base section missing")
	}
	if strings.Contains(output, "=== PROJECT ===") {
		t.Fatalf("project section should be omitted when empty")
	}
	if !strings.Contains(output, "=== INSTANCE ===") {
		t.Fatalf("instance section missing")
	}
	if strings.Contains(output, "=== FRAGMENT ===") {
		t.Fatalf("fragment section should be omitted when empty")
	}
}
