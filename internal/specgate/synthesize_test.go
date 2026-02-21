package specgate

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

type fakeBeadCreator struct {
	createFn func(ctx context.Context, title, description, priority string, labels []string) (string, error)
}

var _ BeadCreator = (*fakeBeadCreator)(nil)

func (f *fakeBeadCreator) Create(ctx context.Context, title, description, priority string, labels []string) (string, error) {
	if f.createFn == nil {
		return "", nil
	}
	return f.createFn(ctx, title, description, priority, labels)
}

func TestSynthesizeFixBeads_zeroFailures(t *testing.T) {
	t.Helper()

	ids, err := SynthesizeFixBeads(context.Background(), "alpha", nil, "P1", nil)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want empty", ids)
	}
}

func TestSynthesizeFixBeads_oneFailure(t *testing.T) {
	t.Helper()

	failures := []CriterionResult{
		{Criterion: "No TODOs", Passed: false, Evidence: "found TODO in internal/specgate/synthesize.go"},
	}

	var gotTitle string
	var gotDescription string
	var gotPriority string
	var gotLabels []string

	creator := &fakeBeadCreator{
		createFn: func(ctx context.Context, title, description, priority string, labels []string) (string, error) {
			gotTitle = title
			gotDescription = description
			gotPriority = priority
			gotLabels = labels
			return "bead-1", nil
		},
	}

	ids, err := SynthesizeFixBeads(context.Background(), "alpha", failures, "P1", creator)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != "bead-1" {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want [bead-1]", ids)
	}
	if gotTitle != "Fix: No TODOs" {
		t.Fatalf("Create title = %q, want %q", gotTitle, "Fix: No TODOs")
	}
	if !strings.Contains(gotDescription, "Criterion: No TODOs") {
		t.Fatalf("Create description = %q, missing criterion", gotDescription)
	}
	if !strings.Contains(gotDescription, "Evidence: found TODO in internal/specgate/synthesize.go") {
		t.Fatalf("Create description = %q, missing evidence", gotDescription)
	}
	if !strings.Contains(gotDescription, "Fix direction:") {
		t.Fatalf("Create description = %q, missing fix direction", gotDescription)
	}
	if gotPriority != "P1" {
		t.Fatalf("Create priority = %q, want %q", gotPriority, "P1")
	}
	if len(gotLabels) != 1 || gotLabels[0] != "spec:alpha" {
		t.Fatalf("Create labels = %v, want [spec:alpha]", gotLabels)
	}
}

func TestSynthesizeFixBeads_fiveFailures(t *testing.T) {
	t.Helper()

	failures := []CriterionResult{
		{Criterion: "one", Passed: false, Evidence: "e1"},
		{Criterion: "two", Passed: false, Evidence: "e2"},
		{Criterion: "three", Passed: false, Evidence: "e3"},
		{Criterion: "four", Passed: false, Evidence: "e4"},
		{Criterion: "five", Passed: false, Evidence: "e5"},
	}

	callCount := 0
	creator := &fakeBeadCreator{
		createFn: func(ctx context.Context, title, description, priority string, labels []string) (string, error) {
			callCount++
			return title, nil
		},
	}

	ids, err := SynthesizeFixBeads(context.Background(), "alpha", failures, "P2", creator)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if callCount != runtypes.MaxSynthesizedSpecFixBeads {
		t.Fatalf("Create call count = %d, want %d", callCount, runtypes.MaxSynthesizedSpecFixBeads)
	}
	if len(ids) != runtypes.MaxSynthesizedSpecFixBeads {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want %d ids", ids, runtypes.MaxSynthesizedSpecFixBeads)
	}
}

func TestSynthesizeFixBeads_sixPlusFailures(t *testing.T) {
	t.Helper()

	failures := []CriterionResult{
		{Criterion: "one", Passed: false, Evidence: "e1"},
		{Criterion: "two", Passed: false, Evidence: "e2"},
		{Criterion: "three", Passed: false, Evidence: "e3"},
		{Criterion: "four", Passed: false, Evidence: "e4"},
		{Criterion: "five", Passed: false, Evidence: "e5"},
		{Criterion: "six", Passed: false, Evidence: "e6"},
	}

	var logBuf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&logBuf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	})

	callCount := 0
	creator := &fakeBeadCreator{
		createFn: func(ctx context.Context, title, description, priority string, labels []string) (string, error) {
			callCount++
			return title, nil
		},
	}

	ids, err := SynthesizeFixBeads(context.Background(), "alpha", failures, "P1", creator)
	if err != nil {
		t.Fatalf("SynthesizeFixBeads() error = %v", err)
	}
	if callCount != runtypes.MaxSynthesizedSpecFixBeads {
		t.Fatalf("Create call count = %d, want %d", callCount, runtypes.MaxSynthesizedSpecFixBeads)
	}
	if len(ids) != runtypes.MaxSynthesizedSpecFixBeads {
		t.Fatalf("SynthesizeFixBeads() ids = %v, want %d ids", ids, runtypes.MaxSynthesizedSpecFixBeads)
	}
	logged := logBuf.String()
	if !strings.Contains(logged, "capped fix bead synthesis") || !strings.Contains(logged, "1 remaining") {
		t.Fatalf("log output = %q, want cap message with remaining count", logged)
	}
}
