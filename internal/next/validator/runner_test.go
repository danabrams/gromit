package validator

import (
	"context"
	"testing"
)

func TestRunCheck_PassingCommand(t *testing.T) {
	r := NewRunner()
	result, err := r.RunCheck(context.Background(), Check{
		Name: "echo", Command: "echo hello", Type: "test",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Fatal("expected pass")
	}
}

func TestRunCheck_FailingCommand(t *testing.T) {
	r := NewRunner()
	result, err := r.RunCheck(context.Background(), Check{
		Name: "fail", Command: "false", Type: "test",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass {
		t.Fatal("expected fail")
	}
}

func TestRunAlwaysRun_AllPass(t *testing.T) {
	r := NewRunner()
	checks := []Check{
		{Name: "echo1", Command: "echo a", Type: "test"},
		{Name: "silent-lint", Command: "true", Type: "lint"},
	}
	results, err := r.RunAlwaysRun(context.Background(), checks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !results.AllPass() {
		t.Fatal("expected all pass")
	}
	if results.PassCount() != 2 {
		t.Fatalf("want 2, got %d", results.PassCount())
	}
}

func TestRunCheck_CommandNotFound_ReturnsError(t *testing.T) {
	r := NewRunner()
	// Use a nonexistent working directory to trigger an exec infrastructure error
	// (sh cannot chdir into a missing directory).
	_, err := r.RunCheck(context.Background(), Check{
		Name: "bad", Command: "echo hi", Type: "test",
	}, "/nonexistent/workdir")
	if err == nil {
		t.Fatal("expected error for command that cannot start")
	}
}

func TestRunCheck_LintWithStdout_FailsEvenOnExitZero(t *testing.T) {
	r := NewRunner()
	// gofmt -l style: exits 0 but lists files needing formatting
	result, err := r.RunCheck(context.Background(), Check{
		Name: "format", Command: "echo 'bad_file.go'", Type: "lint",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Pass {
		t.Fatal("lint check with non-empty stdout should fail")
	}
}

func TestRunCheck_LintWithEmptyStdout_Passes(t *testing.T) {
	r := NewRunner()
	result, err := r.RunCheck(context.Background(), Check{
		Name: "format", Command: "true", Type: "lint",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Fatal("lint check with empty stdout should pass")
	}
}

func TestRunCheck_TestTypeWithStdout_StillPasses(t *testing.T) {
	r := NewRunner()
	// test-type checks should only fail on non-zero exit, not stdout content
	result, err := r.RunCheck(context.Background(), Check{
		Name: "unit", Command: "echo 'ok'", Type: "test",
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Pass {
		t.Fatal("test check with stdout should still pass on exit 0")
	}
}

func TestRunAlwaysRun_SomeFail(t *testing.T) {
	r := NewRunner()
	checks := []Check{
		{Name: "pass", Command: "true", Type: "test"},
		{Name: "fail", Command: "false", Type: "lint"},
	}
	results, err := r.RunAlwaysRun(context.Background(), checks, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if results.AllPass() {
		t.Fatal("expected some failures")
	}
	if results.FailCount() != 1 {
		t.Fatalf("want 1 failure, got %d", results.FailCount())
	}
}
