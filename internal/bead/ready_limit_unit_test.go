package bead

import "testing"

// TestReady_UsesLimit3_Unit is a simple unit test verifying Ready() uses --limit 3
func TestReady_UsesLimit3_Unit(t *testing.T) {
	var capturedArgs []string
	mockRun := func(args ...string) (string, error) {
		capturedArgs = args
		return `[{"id":"task-001","title":"Test","priority":1,"issue_type":"task","status":"open"}]`, nil
	}

	c := &Client{
		binary: "bd",
		RunFn:  mockRun,
	}

	_, err := c.Ready()
	if err != nil {
		t.Fatalf("Ready() error: %v", err)
	}

	// Verify command is: bd ready --json --limit 3
	expected := []string{"ready", "--json", "--limit", "3"}
	if len(capturedArgs) != len(expected) {
		t.Errorf("Ready() called with %d args, want %d\nGot:  %v\nWant: %v",
			len(capturedArgs), len(expected), capturedArgs, expected)
		return
	}

	for i, want := range expected {
		if capturedArgs[i] != want {
			t.Errorf("Ready() arg[%d] = %q, want %q", i, capturedArgs[i], want)
		}
	}
}

// TestReadyWithLabel_UsesLimit3_Unit verifies ReadyWithLabel() uses --limit 3 for consistency
func TestReadyWithLabel_UsesLimit3_Unit(t *testing.T) {
	var capturedArgs []string
	mockRun := func(args ...string) (string, error) {
		capturedArgs = args
		return `[{"id":"task-001","title":"Test","priority":1,"labels":["spec:foo"],"issue_type":"task","status":"open"}]`, nil
	}

	c := &Client{
		binary: "bd",
		RunFn:  mockRun,
	}

	_, err := c.ReadyWithLabel("spec:foo")
	if err != nil {
		t.Fatalf("ReadyWithLabel() error: %v", err)
	}

	// Verify --limit is 3 (not 10) for consistency with Ready()
	limitIdx := -1
	for i, arg := range capturedArgs {
		if arg == "--limit" && i+1 < len(capturedArgs) {
			limitIdx = i + 1
			break
		}
	}

	if limitIdx == -1 {
		t.Fatal("ReadyWithLabel() did not use --limit flag")
	}

	if capturedArgs[limitIdx] != "3" {
		t.Errorf("ReadyWithLabel() uses --limit %s, want --limit 3 for consistency with Ready()",
			capturedArgs[limitIdx])
	}
}
