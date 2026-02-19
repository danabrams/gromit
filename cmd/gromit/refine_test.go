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
