package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/spf13/cobra"
)

func TestShowRefinePickerEOFSelectsNew(t *testing.T) {
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
	}

	choice := showRefinePicker(unrefined, strings.NewReader(""))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for EOF input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerNonNumericInputSelectsNew(t *testing.T) {
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
		{ID: "idea-2", Text: "Second", Type: "feature"},
	}

	choice := showRefinePicker(unrefined, strings.NewReader("abc\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for non-numeric input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerZeroInputSelectsNew(t *testing.T) {
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
	}

	choice := showRefinePicker(unrefined, strings.NewReader("0\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for zero input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerOutOfBoundsHighSelectsNew(t *testing.T) {
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
		{ID: "idea-2", Text: "Second", Type: "feature"},
	}

	// Max valid choice is 3 (len+1 = "something new"), so 99 is out of bounds
	choice := showRefinePicker(unrefined, strings.NewReader("99\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for out-of-bounds input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerValidInputSelectsCorrectItem(t *testing.T) {
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
		{ID: "idea-2", Text: "Second", Type: "feature"},
		{ID: "idea-3", Text: "Third", Type: "bug"},
	}

	// Selecting "2" should return index 1 (Second item)
	choice := showRefinePicker(unrefined, strings.NewReader("2\n"))

	if choice != 1 {
		t.Fatalf("expected choice 1 for input '2', got %d", choice)
	}
}

func TestShowRefinePickerSomethingNewOptionSelectsCorrectly(t *testing.T) {
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
		{ID: "idea-2", Text: "Second", Type: "feature"},
	}

	// Selecting "3" (len+1) is the "Something new..." option
	choice := showRefinePicker(unrefined, strings.NewReader("3\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for 'something new' input, got %d", len(unrefined), choice)
	}
}

func TestShowRefinePickerNegativeInputSelectsNew(t *testing.T) {
	unrefined := []*backlog.Idea{
		{ID: "idea-1", Text: "First", Type: "task"},
	}

	choice := showRefinePicker(unrefined, strings.NewReader("-1\n"))

	if choice != len(unrefined) {
		t.Fatalf("expected choice %d for negative input, got %d", len(unrefined), choice)
	}
}

func TestRunRefineReturnsErrorWhenPipelineCreationFails(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configFile, []byte("loop:\n  max_iterations: 1\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	origConfigPath := configPath
	configPath = configFile
	t.Cleanup(func() {
		configPath = origConfigPath
	})

	origFactory := createRefinePipelineFn
	createRefinePipelineFn = func(_ *config.Config, _, _, _ string) (*pipeline.Pipeline, error) {
		return nil, errors.New("factory failed")
	}
	t.Cleanup(func() {
		createRefinePipelineFn = origFactory
	})

	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")

	err := runRefine(cmd, []string{"ad-hoc idea"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "factory failed") {
		t.Fatalf("expected error to include factory failure, got %v", err)
	}
}
