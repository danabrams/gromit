package reviewpacket

import (
	"testing"
)

func TestProductReview_NormalizeNilFields(t *testing.T) {
	pr := &ProductReview{
		RunID:     "run-1",
		SpecTitle: "Test Spec",
		BehaviorCards: nil,
		Surprises: nil,
	}

	pr.NormalizeNilFields()

	if pr.BehaviorCards == nil {
		t.Errorf("BehaviorCards should be empty slice, not nil")
	}
	if len(pr.BehaviorCards) != 0 {
		t.Errorf("BehaviorCards should be empty, got length %d", len(pr.BehaviorCards))
	}

	if pr.Surprises == nil {
		t.Errorf("Surprises should be empty slice, not nil")
	}
	if len(pr.Surprises) != 0 {
		t.Errorf("Surprises should be empty, got length %d", len(pr.Surprises))
	}
}

func TestBehaviorCard_Creation(t *testing.T) {
	card := &BehaviorCard{
		ID:              "card-1",
		Title:           "Test Card",
		AutomaticStatus: "proven",
		EvidenceFiles:   nil,
		ManualCheckIDs:  nil,
	}

	if card.ID != "card-1" {
		t.Errorf("expected ID 'card-1', got %q", card.ID)
	}
	if card.Title != "Test Card" {
		t.Errorf("expected Title 'Test Card', got %q", card.Title)
	}
}

func TestBehaviorCard_NormalizeNilFields(t *testing.T) {
	card := &BehaviorCard{
		ID:              "card-1",
		Title:           "Test",
		AutomaticStatus: "proven",
		EvidenceFiles:   nil,
		ManualCheckIDs:  nil,
	}

	card.NormalizeNilFields()

	if card.EvidenceFiles == nil {
		t.Errorf("EvidenceFiles should be empty slice, not nil")
	}
	if card.ManualCheckIDs == nil {
		t.Errorf("ManualCheckIDs should be empty slice, not nil")
	}
}

func TestProcessReview_NormalizeNilFields(t *testing.T) {
	pr := &ProcessReview{
		TrustLevel:         "high",
		RepeatedFailureFlag: false,
		RepairCycles:       0,
		DegradedFlags:      nil,
	}

	pr.NormalizeNilFields()

	if pr.DegradedFlags == nil {
		t.Errorf("DegradedFlags should be empty slice, not nil")
	}
	if len(pr.DegradedFlags) != 0 {
		t.Errorf("DegradedFlags should be empty, got length %d", len(pr.DegradedFlags))
	}
}

func TestManualChecklist_NormalizeNilFields(t *testing.T) {
	checklist := &ManualChecklist{
		Items: nil,
	}

	checklist.NormalizeNilFields()

	if checklist.Items == nil {
		t.Errorf("Items should be empty slice, not nil")
	}
	if len(checklist.Items) != 0 {
		t.Errorf("Items should be empty, got length %d", len(checklist.Items))
	}
}

func TestManualCheckItem_NormalizeNilFields(t *testing.T) {
	item := &ManualCheckItem{
		ID:              "check-1",
		Title:           "Verify behavior",
		Instructions:    "Do this thing",
		ExpectedResult:  "Should succeed",
		BehaviorCardIDs: nil,
	}

	item.NormalizeNilFields()

	if item.BehaviorCardIDs == nil {
		t.Errorf("BehaviorCardIDs should be empty slice, not nil")
	}
	if len(item.BehaviorCardIDs) != 0 {
		t.Errorf("BehaviorCardIDs should be empty, got length %d", len(item.BehaviorCardIDs))
	}
}

func TestInputs_Creation(t *testing.T) {
	inputs := &Inputs{
		RunID:     "run-1",
		SpecTitle: "Test Spec",
	}

	if inputs.RunID != "run-1" {
		t.Errorf("expected RunID 'run-1', got %q", inputs.RunID)
	}
	if inputs.SpecTitle != "Test Spec" {
		t.Errorf("expected SpecTitle 'Test Spec', got %q", inputs.SpecTitle)
	}
}

func TestOutputs_Creation(t *testing.T) {
	outputs := &Outputs{
		ProductReview:   ProductReview{RunID: "run-1"},
		ProcessReview:   ProcessReview{TrustLevel: "high"},
		ManualChecklist: ManualChecklist{Items: []ManualCheckItem{}},
	}

	if outputs.ProductReview.RunID != "run-1" {
		t.Errorf("expected ProductReview.RunID 'run-1', got %q", outputs.ProductReview.RunID)
	}
	if outputs.ProcessReview.TrustLevel != "high" {
		t.Errorf("expected ProcessReview.TrustLevel 'high', got %q", outputs.ProcessReview.TrustLevel)
	}
}
