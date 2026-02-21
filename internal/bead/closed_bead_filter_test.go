package bead

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestParseBeadOutputExcluding_SkipsClosedBeads(t *testing.T) {
	beads := []Bead{
		{ID: "closed-1", Title: "Closed task", Priority: 1, Status: "closed", Type: "task"},
		{ID: "open-1", Title: "Open task", Priority: 1, Status: "open", Type: "task"},
	}
	data, err := json.Marshal(beads)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	result, err := parseBeadOutputExcluding(string(data), "epic")
	if err != nil {
		t.Fatalf("parseBeadOutputExcluding error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a bead, got nil")
	}
	if result.ID != "open-1" {
		t.Fatalf("expected open-1, got %s", result.ID)
	}
}

func TestParseBeadOutputExcluding_SkipsClosedBeadsCaseInsensitive(t *testing.T) {
	beads := []Bead{
		{ID: "closed-1", Title: "Closed task", Priority: 1, Status: "Closed", Type: "task"},
		{ID: "closed-2", Title: "Closed task 2", Priority: 1, Status: "CLOSED", Type: "task"},
		{ID: "open-1", Title: "Open task", Priority: 1, Status: "open", Type: "task"},
	}
	data, err := json.Marshal(beads)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	result, err := parseBeadOutputExcluding(string(data), "epic")
	if err != nil {
		t.Fatalf("parseBeadOutputExcluding error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a bead, got nil")
	}
	if result.ID != "open-1" {
		t.Fatalf("expected open-1, got %s", result.ID)
	}
}

func TestParseBeadOutputExcluding_AllClosedReturnsNil(t *testing.T) {
	beads := []Bead{
		{ID: "closed-1", Title: "Closed task", Priority: 1, Status: "closed", Type: "task"},
		{ID: "closed-2", Title: "Closed task 2", Priority: 1, Status: "closed", Type: "task"},
	}
	data, err := json.Marshal(beads)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	result, err := parseBeadOutputExcluding(string(data), "epic")
	if err != nil {
		t.Fatalf("parseBeadOutputExcluding error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got bead %s", result.ID)
	}
}

func TestReadyExcluding_SkipsClosedBeads(t *testing.T) {
	beads := []Bead{
		{ID: "closed-1", Title: "Closed task", Priority: 1, Status: "closed", Type: "task"},
		{ID: "open-1", Title: "Open task", Priority: 1, Status: "open", Type: "task"},
	}
	data, err := json.Marshal(beads)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	c := &Client{
		binary: "bd",
		RunFn: func(args ...string) (string, error) {
			return string(data), nil
		},
	}

	result, err := c.ReadyExcluding(map[string]bool{})
	if err != nil {
		t.Fatalf("ReadyExcluding error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a bead, got nil")
	}
	if result.ID != "open-1" {
		t.Fatalf("expected open-1, got %s", result.ID)
	}
}

func TestReadyExcluding_SkipsClosedAndExcludedBeads(t *testing.T) {
	beads := []Bead{
		{ID: "closed-1", Title: "Closed task", Priority: 1, Status: "closed", Type: "task"},
		{ID: "excluded-1", Title: "Excluded task", Priority: 1, Status: "open", Type: "task"},
		{ID: "open-1", Title: "Open task", Priority: 1, Status: "open", Type: "task"},
	}
	data, err := json.Marshal(beads)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	c := &Client{
		binary: "bd",
		RunFn: func(args ...string) (string, error) {
			return string(data), nil
		},
	}

	result, err := c.ReadyExcluding(map[string]bool{"excluded-1": true})
	if err != nil {
		t.Fatalf("ReadyExcluding error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a bead, got nil")
	}
	if result.ID != "open-1" {
		t.Fatalf("expected open-1, got %s", result.ID)
	}
}

func TestReadyWithLabel_SkipsClosedBeadFromShow(t *testing.T) {
	// parseBeadOutputExcluding returns a bead that has the right label
	// but Show reveals it is actually closed
	readyOutput := []Bead{
		{ID: "bead-1", Title: "Task with label", Priority: 1, Status: "open", Type: "task", Labels: []string{}},
	}
	readyData, err := json.Marshal(readyOutput)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	c := &Client{
		binary: "bd",
		RunFn: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == "show" {
				// bd show returns this bead as closed
				showBead := Bead{
					ID:       "bead-1",
					Title:    "Task with label",
					Priority: 1,
					Status:   "closed",
					Type:     "task",
					Labels:   []string{"spec:foo"},
				}
				data, _ := json.Marshal([]Bead{showBead})
				return string(data), nil
			}
			return string(readyData), nil
		},
	}

	result, err := c.ReadyWithLabel("spec:foo")
	if err != nil {
		t.Fatalf("ReadyWithLabel error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for closed bead from show, got bead %s", result.ID)
	}
}

func TestReady_SkipsClosedBeads(t *testing.T) {
	beads := []Bead{
		{ID: "closed-1", Title: "Closed task", Priority: 1, Status: "closed", Type: "task"},
		{ID: "open-1", Title: "Open task", Priority: 1, Status: "open", Type: "task"},
	}
	data, err := json.Marshal(beads)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	c := &Client{
		binary: "bd",
		RunFn: func(args ...string) (string, error) {
			return string(data), nil
		},
	}

	result, err := c.Ready()
	if err != nil {
		t.Fatalf("Ready error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a bead, got nil")
	}
	if result.ID != "open-1" {
		t.Fatalf("expected open-1, got %s", result.ID)
	}
}

func TestReadyExcluding_AllClosedReturnsNil(t *testing.T) {
	beads := []Bead{
		{ID: "closed-1", Title: "Closed 1", Priority: 1, Status: "closed", Type: "task"},
		{ID: "closed-2", Title: "Closed 2", Priority: 1, Status: "closed", Type: "task"},
	}
	data, err := json.Marshal(beads)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	c := &Client{
		binary: "bd",
		RunFn: func(args ...string) (string, error) {
			return string(data), nil
		},
	}

	result, err := c.ReadyExcluding(map[string]bool{})
	if err != nil {
		t.Fatalf("ReadyExcluding error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil when all beads are closed, got %s", result.ID)
	}
}

func TestReadyExcluding_DelegatesToReadyWhenNoExcludes(t *testing.T) {
	// When excludeIDs is empty, ReadyExcluding delegates to Ready,
	// which calls parseBeadOutputExcluding. Verify closed beads are still filtered.
	beads := []Bead{
		{ID: "closed-1", Title: "Closed task", Priority: 1, Status: "closed", Type: "task"},
		{ID: "open-1", Title: "Open task", Priority: 1, Status: "open", Type: "task"},
	}
	data, err := json.Marshal(beads)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	c := &Client{
		binary: "bd",
		RunFn: func(args ...string) (string, error) {
			return string(data), nil
		},
	}

	// nil map triggers delegation to Ready()
	result, err := c.ReadyExcluding(nil)
	if err != nil {
		t.Fatalf("ReadyExcluding(nil) error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a bead, got nil")
	}
	if result.ID != "open-1" {
		t.Fatalf("expected open-1, got %s", result.ID)
	}
}

func TestReadyWithLabel_SkipsClosedBeadFromParse(t *testing.T) {
	// All beads from bd ready are closed
	beads := []Bead{
		{ID: "closed-1", Title: "Closed task", Priority: 1, Status: "closed", Type: "task", Labels: []string{"spec:foo"}},
	}
	data, err := json.Marshal(beads)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	showCalled := false
	c := &Client{
		binary: "bd",
		RunFn: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == "show" {
				showCalled = true
				return "", fmt.Errorf("should not be called")
			}
			return string(data), nil
		},
	}

	result, err := c.ReadyWithLabel("spec:foo")
	if err != nil {
		t.Fatalf("ReadyWithLabel error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for closed bead, got bead %s", result.ID)
	}
	if showCalled {
		t.Fatal("Show should not be called when parseBeadOutputExcluding returns nil")
	}
}
