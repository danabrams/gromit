package prompt

import "testing"

func TestContextExperimentAndVariantIDsHoldValues(t *testing.T) {
    ctx := &Context{
        ExperimentID: "exp-123",
        VariantID:    "var-456",
    }

    if ctx.ExperimentID != "exp-123" {
        t.Fatalf("ExperimentID = %q, want %q", ctx.ExperimentID, "exp-123")
    }
    if ctx.VariantID != "var-456" {
        t.Fatalf("VariantID = %q, want %q", ctx.VariantID, "var-456")
    }
}
