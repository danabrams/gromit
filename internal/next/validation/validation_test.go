package validation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCommand_String(t *testing.T) {
	cmd := Command{
		Name:   "go-test",
		Kind:   Test,
		Run:    "go test ./...",
		Source: "Makefile",
	}
	if cmd.Name != "go-test" {
		t.Errorf("Name = %q, want %q", cmd.Name, "go-test")
	}
	if cmd.Kind != Test {
		t.Errorf("Kind = %v, want Test", cmd.Kind)
	}
}

func TestKind_String(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{Test, "test"},
		{Lint, "lint"},
		{Build, "build"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestCommandSet_Add(t *testing.T) {
	cs := NewCommandSet()
	cs.Add(Command{Name: "test", Kind: Test, Run: "go test ./...", Source: "Makefile"})
	cs.Add(Command{Name: "lint", Kind: Lint, Run: "golangci-lint run", Source: "Makefile"})

	if len(cs.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(cs.Commands))
	}
}

func TestCommandSet_ByKind(t *testing.T) {
	cs := NewCommandSet()
	cs.Add(Command{Name: "unit", Kind: Test, Run: "go test ./...", Source: "Makefile"})
	cs.Add(Command{Name: "lint", Kind: Lint, Run: "golangci-lint run", Source: "Makefile"})
	cs.Add(Command{Name: "integ", Kind: Test, Run: "go test -tags=integration ./...", Source: "CI"})

	tests := cs.ByKind(Test)
	if len(tests) != 2 {
		t.Errorf("expected 2 test commands, got %d", len(tests))
	}
}

func TestKind_JSONRoundTrip(t *testing.T) {
	cmd := Command{Name: "test", Kind: Test, Run: "go test ./...", Source: "Makefile"}
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"test"`) {
		t.Errorf("expected JSON to contain \"test\", got %s", data)
	}
	var cmd2 Command
	if err := json.Unmarshal(data, &cmd2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cmd2.Kind != Test {
		t.Errorf("Kind = %v, want Test", cmd2.Kind)
	}
}

func TestNewCommandSet_InitializesEmpty(t *testing.T) {
	cs := NewCommandSet()
	if cs.Commands == nil {
		t.Error("Commands should be initialized, not nil")
	}
}
