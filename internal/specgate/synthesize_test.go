package specgate

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeBeadCreator struct {
	createFn func(ctx context.Context, title, description, priority string, labels []string) (string, error)
}

func (f *fakeBeadCreator) Create(ctx context.Context, title, description, priority string, labels []string) (string, error) {
	if f.createFn == nil {
		return "", nil
	}
	return f.createFn(ctx, title, description, priority, labels)
}

func TestSynthesizeFixBeads_createsBeadsForFailures(t *testing.T) {
	ctx := context.Background()
	failures := []CriterionResult{{Criterion: "No TODOs", Passed: false, Evidence: "found TODO in file"}}
	creator := &fakeBeadCreator{
		createFn: func(ctx context.Context, title, description, priority string, labels []string) (string, error) {
			if title != "No TODOs" {
				t.Fatalf("Create title = %q, want %q", title, "No TODOs")
			}
			wantDescription := "Criterion: No TODOs\nEvidence: found TODO in file"
			if description != wantDescription {
				t.Fatalf("Create description = %q, want %q", description, wantDescription)
			}
			if priority != "P1" {
				t.Fatalf("Create priority = %q, want %q", priority, "P1")
			}
			if len(labels) != 1 || labels[0] != "spec:alpha" {
				t.Fatalf("Create labels = %v, want [spec:alpha]", labels)
			}
			return "bead-1", nil
		},
	}

	ids, err := SynthesizeFixBeads(ctx, "alpha", failures, "P1", creator)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != "bead-1" {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want [bead-1]", ids)
	}
}

func TestSynthesizeFixBeads_truncatesTitle(t *testing.T) {
	ctx := context.Background()
	longTitle := strings.Repeat("a", 85)
	creator := &fakeBeadCreator{
		createFn: func(ctx context.Context, title, description, priority string, labels []string) (string, error) {
			if len(title) != 80 {
				t.Fatalf("Create title length = %d, want 80", len(title))
			}
			if title != strings.Repeat("a", 80) {
				t.Fatalf("Create title = %q, want %q", title, strings.Repeat("a", 80))
			}
			return "bead-1", nil
		},
	}

	_, err := SynthesizeFixBeads(ctx, "alpha", []CriterionResult{{Criterion: longTitle, Passed: false}}, "P1", creator)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
}

func TestSynthesizeFixBeads_noFailures_doesNotRequireCreator(t *testing.T) {
	ctx := context.Background()

	ids, err := SynthesizeFixBeads(ctx, "alpha", nil, "P1", nil)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want empty slice", ids)
	}
}

func TestSynthesizeFixBeads_createsFiveBeadsForFiveFailures(t *testing.T) {
	ctx := context.Background()
	failures := []CriterionResult{
		{Criterion: "one", Passed: false},
		{Criterion: "two", Passed: false},
		{Criterion: "three", Passed: false},
		{Criterion: "four", Passed: false},
		{Criterion: "five", Passed: false},
	}

	callCount := 0
	creator := &fakeBeadCreator{
		createFn: func(ctx context.Context, title, description, priority string, labels []string) (string, error) {
			callCount++
			return title, nil
		},
	}

	ids, err := SynthesizeFixBeads(ctx, "alpha", failures, "P2", creator)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if len(ids) != 5 {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want 5 items", ids)
	}
	if callCount != 5 {
		t.Fatalf("Create callCount = %d, want 5", callCount)
	}
}

func TestSynthesizeFixBeads_capsAtFiveBeadsForSixPlusFailures(t *testing.T) {
	ctx := context.Background()
	failures := []CriterionResult{
		{Criterion: "one", Passed: false},
		{Criterion: "two", Passed: false},
		{Criterion: "three", Passed: false},
		{Criterion: "four", Passed: false},
		{Criterion: "five", Passed: false},
		{Criterion: "six", Passed: false},
		{Criterion: "seven", Passed: false},
	}
	callCount := 0
	creator := &fakeBeadCreator{
		createFn: func(ctx context.Context, title, description, priority string, labels []string) (string, error) {
			callCount++
			if callCount > 5 {
				t.Fatalf("Create called %d times, want at most 5", callCount)
			}
			return title, nil
		},
	}

	ids, err := SynthesizeFixBeads(ctx, "alpha", failures, "P1", creator)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if len(ids) != 5 {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want 5 items", ids)
	}
}

func TestSynthesizeFixBeads_creatorError(t *testing.T) {
	ctx := context.Background()
	failures := []CriterionResult{
		{Criterion: "No TODOs", Passed: false, Evidence: "found TODO"},
		{Criterion: "No panics", Passed: false, Evidence: "panic in helper"},
	}
	wantErr := errors.New("create failed")
	callCount := 0
	creator := &fakeBeadCreator{
		createFn: func(ctx context.Context, title, description, priority string, labels []string) (string, error) {
			callCount++
			if callCount == 1 {
				return "bead-1", nil
			}
			return "", wantErr
		},
	}

	ids, err := SynthesizeFixBeads(ctx, "alpha", failures, "P1", creator)
	if !errors.Is(err, wantErr) {
		t.Fatalf("SynthesizeFixBeads() error = %v, want %v", err, wantErr)
	}
	if len(ids) != 1 || ids[0] != "bead-1" {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want [bead-1]", ids)
	}
}
