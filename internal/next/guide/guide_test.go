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

func TestRenderInput_NormalizeNilFields(t *testing.T) {
	input := RenderInput{ProjectName: "test"}
	// All slice fields should be nil initially.
	input.NormalizeNilFields()

	if input.Modules == nil || len(input.Modules) != 0 {
		t.Error("Modules should be empty non-nil")
	}
	if input.SourceMap == nil || len(input.SourceMap) != 0 {
		t.Error("SourceMap should be empty non-nil")
	}
	if input.Validation == nil || len(input.Validation) != 0 {
		t.Error("Validation should be empty non-nil")
	}
	if input.Risks == nil || len(input.Risks) != 0 {
		t.Error("Risks should be empty non-nil")
	}
	if input.Invariants == nil || len(input.Invariants) != 0 {
		t.Error("Invariants should be empty non-nil")
	}
	if input.Glossary == nil || len(input.Glossary) != 0 {
		t.Error("Glossary should be empty non-nil")
	}
	if input.Doctrine == nil || len(input.Doctrine) != 0 {
		t.Error("Doctrine should be empty non-nil")
	}
}

func TestMarkdownRenderer_AllSections(t *testing.T) {
	r := NewMarkdownRenderer()
	input := RenderInput{
		ProjectName: "full-guide",
		Modules: []Module{
			{Name: "cmd/api", Description: "HTTP entrypoint", Language: "go"},
		},
		SourceMap: []SourceMapEntry{
			{Path: "main.go", Language: "go", Lines: 50},
		},
		Validation: []ValidationCommand{
			{Name: "test", Kind: "test", Run: "go test ./...", Source: "Makefile"},
		},
		Risks: []Risk{
			{Area: "auth", Description: "token expiry", Severity: "high"},
		},
		Invariants: []Invariant{
			{Rule: "no-cycles", Description: "no import cycles", Scope: "global"},
		},
		Glossary: []GlossaryEntry{
			{Term: "PCI", Definition: "Payment Card Industry"},
		},
		Doctrine: []DoctrineRule{
			{ID: "D1", Summary: "test first", Scope: "global"},
		},
	}

	output, err := r.Render(input)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	md := string(output)

	for _, heading := range []string{
		"## Architecture",
		"## Source Map",
		"## Validation",
		"## Risky Areas",
		"## Invariants",
		"## Glossary",
		"## Doctrine",
	} {
		if !strings.Contains(md, heading) {
			t.Errorf("missing section heading %q", heading)
		}
	}
}

func TestMarkdownRenderer_InferredSections(t *testing.T) {
	r := NewMarkdownRenderer()
	input := RenderInput{
		ProjectName: "test-project",
		InferredFacts: []InferredObservation{
			{Category: "component_boundary", Statement: "API layer is separate from storage", Confidence: "high"},
			{Category: "risky_area", Statement: "No error handling in webhook handler", Confidence: "medium"},
			{Category: "glossary_term", Statement: "Bead: a unit of work in the pipeline", Confidence: "high"},
		},
		IncludeInferred: true,
	}

	out, err := r.Render(input)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)

	if !strings.Contains(s, "[INFERRED]") {
		t.Error("inferred content should be marked with [INFERRED]")
	}
	if !strings.Contains(s, "Inferred Component Structure") {
		t.Error("expected inferred component structure section")
	}
	if !strings.Contains(s, "Inferred Risky Areas") {
		t.Error("expected inferred risky areas section")
	}
}

func TestMarkdownRenderer_NoInferredByDefault(t *testing.T) {
	r := NewMarkdownRenderer()
	input := RenderInput{
		ProjectName: "test-project",
		InferredFacts: []InferredObservation{
			{Category: "entrypoint", Statement: "main.go", Confidence: "high"},
		},
		IncludeInferred: false,
	}

	out, _ := r.Render(input)
	if strings.Contains(string(out), "[INFERRED]") {
		t.Error("inferred content should not appear when IncludeInferred is false")
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
