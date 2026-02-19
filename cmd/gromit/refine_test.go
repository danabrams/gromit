package main

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
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
