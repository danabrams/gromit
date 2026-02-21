package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTaggedHarnessContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testDir := filepath.Join(root, "test")
	contractsDir := filepath.Join(testDir, "contracts")
	e2eDir := filepath.Join(testDir, "e2e")
	fakesDir := filepath.Join(testDir, "fakes")
	fixturesDir := filepath.Join(testDir, "fixtures")
	cmdDir := filepath.Join(root, "cmd", "gromit")

	for _, dir := range []string{contractsDir, e2eDir, fakesDir, fixturesDir, cmdDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	ctx, err := ResolveTaggedHarnessContext(contractsDir)
	if err != nil {
		t.Fatalf("ResolveTaggedHarnessContext returned error: %v", err)
	}
	if ctx.WorkingDir != contractsDir {
		t.Fatalf("WorkingDir = %q, want %q", ctx.WorkingDir, contractsDir)
	}
	if ctx.FakesDir != fakesDir {
		t.Fatalf("FakesDir = %q, want %q", ctx.FakesDir, fakesDir)
	}
	if ctx.FixturesDir != fixturesDir {
		t.Fatalf("FixturesDir = %q, want %q", ctx.FixturesDir, fixturesDir)
	}
	if ctx.CmdGromitDir != cmdDir {
		t.Fatalf("CmdGromitDir = %q, want %q", ctx.CmdGromitDir, cmdDir)
	}
}

func TestResolveTaggedHarnessContext_FromRepoRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testDir := filepath.Join(root, "test")
	contractsDir := filepath.Join(testDir, "contracts")
	e2eDir := filepath.Join(testDir, "e2e")
	fakesDir := filepath.Join(testDir, "fakes")
	fixturesDir := filepath.Join(testDir, "fixtures")
	cmdDir := filepath.Join(root, "cmd", "gromit")

	for _, dir := range []string{contractsDir, e2eDir, fakesDir, fixturesDir, cmdDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	ctx, err := ResolveTaggedHarnessContext(root)
	if err != nil {
		t.Fatalf("ResolveTaggedHarnessContext returned error: %v", err)
	}
	if ctx.WorkingDir != root {
		t.Fatalf("WorkingDir = %q, want %q", ctx.WorkingDir, root)
	}
	if ctx.FakesDir != fakesDir {
		t.Fatalf("FakesDir = %q, want %q", ctx.FakesDir, fakesDir)
	}
	if ctx.FixturesDir != fixturesDir {
		t.Fatalf("FixturesDir = %q, want %q", ctx.FixturesDir, fixturesDir)
	}
	if ctx.CmdGromitDir != cmdDir {
		t.Fatalf("CmdGromitDir = %q, want %q", ctx.CmdGromitDir, cmdDir)
	}
}

func TestResolveTaggedHarnessContext_RejectsUnexpectedWorkingDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	badDir := filepath.Join(root, "tmp", "other")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", badDir, err)
	}

	_, err := ResolveTaggedHarnessContext(badDir)
	if err == nil {
		t.Fatal("expected error for unexpected working directory")
	}
	if !strings.Contains(err.Error(), "unable to resolve tagged harness repo root") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "attempted:") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveTaggedHarnessContext_ValidatesRequiredDirs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	e2eDir := filepath.Join(root, "test", "e2e")
	contractsDir := filepath.Join(root, "test", "contracts")
	if err := os.MkdirAll(e2eDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", e2eDir, err)
	}
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", contractsDir, err)
	}

	_, err := ResolveTaggedHarnessContext(e2eDir)
	if err == nil {
		t.Fatal("expected error when required directories are missing")
	}
	if !strings.Contains(err.Error(), "unable to resolve tagged harness repo root") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "attempted:") {
		t.Fatalf("unexpected error: %v", err)
	}
}
