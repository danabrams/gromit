package wiring_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	wiringstage "github.com/danabrams/gromit/internal/pipeline/qualitygate/wiring"
)

func TestWiringGate_SkipsWhenDisabled(t *testing.T) {
	stage := wiringstage.New(func(context.Context) (string, error) {
		t.Fatal("git diff should not run when wiring gate is disabled")
		return "", nil
	})

	input := pipeline.Input{
		Config: &config.Config{
			WiringGate: config.WiringGateConfig{Enabled: false},
		},
	}

	output, err := stage.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if output.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want %v", output.Decision, pipeline.Proceed)
	}
	if len(output.WiringFailures) != 0 {
		t.Fatalf("WiringFailures = %v, want none", output.WiringFailures)
	}
}

func TestWiringGate_BlocksUnwiredSymbol(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte("package main\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	diff := strings.Join([]string{
		"diff --git a/foo.go b/foo.go",
		"new file mode 100644",
		"index 0000000..1111111",
		"--- /dev/null",
		"+++ b/foo.go",
		"+package main",
		"+",
		"+func Foo() {}",
	}, "\n")

	stage := wiringstage.New(func(context.Context) (string, error) {
		return diff, nil
	})

	input := pipeline.Input{
		Bead: &bead.Bead{ID: "bead-1"},
		Config: &config.Config{
			WiringGate: config.WiringGateConfig{Enabled: true},
		},
	}

	output, err := stage.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if output.Decision != pipeline.Block {
		t.Fatalf("Decision = %v, want %v", output.Decision, pipeline.Block)
	}
	if len(output.WiringFailures) != 1 || output.WiringFailures[0] != "Foo exported but not referenced" {
		t.Fatalf("WiringFailures = %v, want [Foo exported but not referenced]", output.WiringFailures)
	}
}

func TestWiringGate_AllSymbolsWired(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte("package main\n\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bar.go"), []byte("package main\n\nfunc UseFoo() { Foo() }\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	diff := strings.Join([]string{
		"diff --git a/foo.go b/foo.go",
		"new file mode 100644",
		"index 0000000..1111111",
		"--- /dev/null",
		"+++ b/foo.go",
		"+package main",
		"+",
		"+func Foo() {}",
	}, "\n")

	stage := wiringstage.New(func(context.Context) (string, error) {
		return diff, nil
	})

	input := pipeline.Input{
		Bead: &bead.Bead{ID: "bead-1"},
		Config: &config.Config{
			WiringGate: config.WiringGateConfig{Enabled: true},
		},
	}

	output, err := stage.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if output.Decision != pipeline.Proceed {
		t.Fatalf("Decision = %v, want %v", output.Decision, pipeline.Proceed)
	}
	if len(output.WiringFailures) != 0 {
		t.Fatalf("WiringFailures = %v, want none", output.WiringFailures)
	}
}
