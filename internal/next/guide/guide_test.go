package guide

import (
	"strings"
	"testing"
)

func TestMarkdownRenderer_Render(t *testing.T) {
	r := NewMarkdownRenderer()
	input := RenderInput{
		ProjectName: "payments-api",
		Glossary: []GlossaryEntry{
			{Term: "PCI", Definition: "Payment Card Industry compliance standard"},
		},
	}

	output, err := r.Render(input)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	md := string(output)
	if !strings.Contains(md, "# payments-api") {
		t.Error("missing project heading")
	}
	if !strings.Contains(md, "## Project Overview") {
		t.Error("missing Project Overview section")
	}
	if !strings.Contains(md, "PCI") {
		t.Error("missing glossary entry")
	}
}

func TestMarkdownRenderer_OmitsEmptySections(t *testing.T) {
	r := NewMarkdownRenderer()
	input := RenderInput{
		ProjectName: "minimal",
	}

	output, _ := r.Render(input)
	md := string(output)
	if strings.Contains(md, "## Glossary") {
		t.Error("should omit empty Glossary section")
	}
	if strings.Contains(md, "## Risky Areas") {
		t.Error("should omit empty Risky Areas section")
	}
	if !strings.Contains(md, "## Project Overview") {
		t.Error("Project Overview should always be present")
	}
}
