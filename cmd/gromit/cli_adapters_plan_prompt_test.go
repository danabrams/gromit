package main

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

func TestPlanPromptRenderer_RenderPlanReportsNotAvailable(t *testing.T) {
	t.Parallel()

	adapter := &planPromptRenderer{}
	_, err := adapter.RenderPlan(&pipeline.PlanPromptInput{IdeaText: "some spec"})

	if err == nil {
		t.Fatalf("expected error indicating plan rendering is unavailable, got nil")
	}

	if !strings.Contains(err.Error(), "not yet available") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRefinePromptRenderer_RenderRefineReportsNotAvailable(t *testing.T) {
	t.Parallel()

	adapter := &refinePromptRenderer{}
	_, err := adapter.RenderRefine(&pipeline.RefinePromptInput{IdeaText: "a thing"})

	if err == nil {
		t.Fatalf("expected error indicating refine rendering is unavailable, got nil")
	}

	if !strings.Contains(err.Error(), "not yet available") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
