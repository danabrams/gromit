package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestReviewInteractivePathUsesModelFlag(t *testing.T) {
	t.Parallel()

	// Create a test command with the model flag
	cmd := &cobra.Command{Use: "review"}
	cmd.Flags().String("model", "opus", "model override")

	// Verify the flag exists
	flag := cmd.Flags().Lookup("model")
	if flag == nil {
		t.Fatal("model flag not found on test command")
	}

	// Set the flag to a non-default value
	if err := cmd.Flags().Set("model", "sonnet"); err != nil {
		t.Fatalf("setting model flag: %v", err)
	}

	// Test that resolveInteractiveModel can get the value
	modelValue := resolveInteractiveModel(cmd, "model")
	if modelValue != "sonnet" {
		t.Errorf("resolveInteractiveModel() = %q, want %q", modelValue, "sonnet")
	}

	// Test that the flag was changed
	if !cmd.Flags().Changed("model") {
		t.Error("expected model flag to be marked as changed")
	}
}

func TestRefineInteractivePathUsesModelFlag(t *testing.T) {
	t.Parallel()

	// Verify the refine command has the model flag
	flag := refineCmd.Flags().Lookup("model")
	if flag == nil {
		t.Fatal("refine command missing model flag")
	}

	// Test that resolveInteractiveModel can get the default value
	modelValue := resolveInteractiveModel(refineCmd, "model")
	if modelValue != "opus" {
		t.Errorf("default model = %q, want %q", modelValue, "opus")
	}
}

func TestPlanInteractivePathUsesModelFlag(t *testing.T) {
	t.Parallel()

	// Verify the plan command has the model flag
	flag := planCmd.Flags().Lookup("model")
	if flag == nil {
		t.Fatal("plan command missing model flag")
	}

	// Test that resolveInteractiveModel can get the default value
	modelValue := resolveInteractiveModel(planCmd, "model")
	if modelValue != "opus" {
		t.Errorf("default model = %q, want %q", modelValue, "opus")
	}
}
