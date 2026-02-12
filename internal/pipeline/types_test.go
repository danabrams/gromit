package pipeline

import "testing"

func TestEvent_Creation(t *testing.T) {
	event := Event{
		Type:    EventOutput,
		Content: "test output",
	}

	if event.Type != EventOutput {
		t.Errorf("Type = %v, want %v", event.Type, EventOutput)
	}
	if event.Content != "test output" {
		t.Errorf("Content = %q, want %q", event.Content, "test output")
	}
}

func TestEventType_Constants(t *testing.T) {
	if EventOutput != 0 {
		t.Errorf("EventOutput = %d, want 0", EventOutput)
	}
	if EventSessionStarted != 1 {
		t.Errorf("EventSessionStarted = %d, want 1", EventSessionStarted)
	}
	if EventSessionEnded != 2 {
		t.Errorf("EventSessionEnded = %d, want 2", EventSessionEnded)
	}
	if EventError != 3 {
		t.Errorf("EventError = %d, want 3", EventError)
	}
}

func TestConstructors_InitializeSlices(t *testing.T) {
	// Constructors should return structs with non-nil slices

	t.Run("RefineResult", func(t *testing.T) {
		result := NewRefineResult()
		if result.CreatedSpecs == nil {
			t.Error("NewRefineResult().CreatedSpecs should be non-nil")
		}
		if result.RefinedItems == nil {
			t.Error("NewRefineResult().RefinedItems should be non-nil")
		}
	})

	t.Run("PlanResult", func(t *testing.T) {
		result := NewPlanResult()
		if result.CreatedPlans == nil {
			t.Error("NewPlanResult().CreatedPlans should be non-nil")
		}
	})

	t.Run("DecomposeResult", func(t *testing.T) {
		result := NewDecomposeResult()
		if result.CreatedBeads == nil {
			t.Error("NewDecomposeResult().CreatedBeads should be non-nil")
		}
	})

	t.Run("ReviewResult", func(t *testing.T) {
		result := NewReviewResult()
		if result.CreatedBeads == nil {
			t.Error("NewReviewResult().CreatedBeads should be non-nil")
		}
		if result.CreatedBacklogItems == nil {
			t.Error("NewReviewResult().CreatedBacklogItems should be non-nil")
		}
	})

	t.Run("ExploreResult", func(t *testing.T) {
		result := NewExploreResult()
		if result.CreatedSpecs == nil {
			t.Error("NewExploreResult().CreatedSpecs should be non-nil")
		}
		if result.CreatedEpics == nil {
			t.Error("NewExploreResult().CreatedEpics should be non-nil")
		}
		if result.CreatedBacklogItems == nil {
			t.Error("NewExploreResult().CreatedBacklogItems should be non-nil")
		}
	})

	t.Run("CreatedBead", func(t *testing.T) {
		bead := NewCreatedBead()
		if bead.Labels == nil {
			t.Error("NewCreatedBead().Labels should be non-nil")
		}
	})
}
