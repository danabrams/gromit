//go:build acceptance

package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestAutoFixProductionWiring_NewRunnerWiresRealAutoFixFn verifies that
// NewRunner() wires a real autoFixFn implementation that runs gofmt and goimports
// on changed files, rather than leaving autoFixFn as nil.
//
// Acceptance criterion: NewRunner() initializes r.autoFixFn with a production
// implementation that calls gofmt -w and goimports -w on changed files.
//
// Expected failure: NewRunner() does not currently wire autoFixFn — the field
// is only set in test code. The validation runner receives nil for autoFixFn,
// so auto-fix never runs in production.
func TestAutoFixProductionWiring_NewRunnerWiresRealAutoFixFn(t *testing.T) {
	cfg := &config.Config{
		Paths: config.PathConfig{
			Templates:       ".gromit/templates",
			Specs:           ".gromit/specs",
			Logs:            ".gromit/logs",
			ProjectClaudeMD: "CLAUDE.md",
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	cfg.SetDefaults()

	var output strings.Builder
	r, err := NewRunner(cfg, &output)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	if r.autoFixFn == nil {
		t.Error("NewRunner did not wire autoFixFn — production builds will skip auto-fix entirely")
	}

	// Verify the wired autoFixFn is callable (implementation test happens elsewhere)
	if r.autoFixFn != nil {
		// Call with a fake commit to ensure it doesn't panic
		_ = r.autoFixFn("abc123")
	}
}

// TestAutoFixProductionWiring_NewRunnerWithDepsWiresRealAutoFixFn verifies that
// NewRunnerWithDeps() also wires a real autoFixFn when no explicit autoFixFn
// is provided in the deps, ensuring production consistency across both constructors.
//
// Acceptance criterion: NewRunnerWithDeps() initializes r.autoFixFn with the same
// production implementation as NewRunner() when autoFixFn is not explicitly provided.
//
// Expected failure: NewRunnerWithDeps() does not currently wire autoFixFn —
// it's only set when explicitly provided via Deps, which is a test-only scenario.
func TestAutoFixProductionWiring_NewRunnerWithDepsWiresRealAutoFixFn(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	cfg.SetDefaults()

	var output strings.Builder
	deps := Deps{
		Beads:    &mockBeadClient{},
		Router:   nil,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockRenderer{},
		Logger:   nil,
	}

	r, err := NewRunnerWithDeps(cfg, &output, ".gromit", deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	if r.autoFixFn == nil {
		t.Error("NewRunnerWithDeps did not wire autoFixFn — production builds via this path will skip auto-fix entirely")
	}
}

// TestAutoFixProductionWiring_AutoFixFnRunsGofmtAndGoimports verifies that the
// production autoFixFn implementation actually runs gofmt -w and goimports -w
// on changed files, not just a no-op stub.
//
// Acceptance criterion: The wired autoFixFn calls "gofmt -w" and "goimports -w"
// on Go files that changed since the provided startCommit.
//
// Expected failure: No production implementation of autoFixFn exists yet. Tests
// have been using mock implementations, and NewRunner/NewRunnerWithDeps leave
// autoFixFn as nil, so this functionality doesn't exist in production code.
func TestAutoFixProductionWiring_AutoFixFnRunsGofmtAndGoimports(t *testing.T) {
	// Set up a test git repository
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	// Initialize git repo
	runCmd(t, "git", "init")
	runCmd(t, "git", "config", "user.email", "test@test.com")
	runCmd(t, "git", "config", "user.name", "Test User")

	// Create a badly formatted Go file
	badlyFormattedGo := `package main
import "fmt"
var x    =    1
func main(){fmt.Println(x)}`

	goFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(goFile, []byte(badlyFormattedGo), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Commit the bad file
	runCmd(t, "git", "add", ".")
	runCmd(t, "git", "commit", "-m", "initial commit")

	// Get the commit hash
	startCommit := strings.TrimSpace(runCmd(t, "git", "rev-parse", "HEAD"))

	// Modify the file to be even worse
	worseFormatted := `package main
import "fmt"
var x      =        1
func main(  ){fmt.Println(    x   )}`
	if err := os.WriteFile(goFile, []byte(worseFormatted), 0644); err != nil {
		t.Fatalf("failed to write modified file: %v", err)
	}

	// Create runner and get its autoFixFn
	cfg := &config.Config{
		Paths: config.PathConfig{
			Templates:       ".gromit/templates",
			Specs:           ".gromit/specs",
			Logs:            ".gromit/logs",
			ProjectClaudeMD: "CLAUDE.md",
		},
	}
	cfg.SetDefaults()

	var output strings.Builder
	r, err := NewRunner(cfg, &output)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	if r.autoFixFn == nil {
		t.Fatal("autoFixFn is nil — cannot test its behavior")
	}

	// Call autoFixFn
	if err := r.autoFixFn(startCommit); err != nil {
		t.Errorf("autoFixFn failed: %v", err)
	}

	// Verify the file was formatted
	formatted, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("failed to read formatted file: %v", err)
	}

	formattedStr := string(formatted)

	// Check for gofmt corrections (spacing, braces)
	if strings.Contains(formattedStr, "main(  )") {
		t.Error("autoFixFn did not run gofmt — excessive whitespace still present in function signature")
	}
	if strings.Contains(formattedStr, "x      =        1") {
		t.Error("autoFixFn did not run gofmt — excessive whitespace still present in variable declaration")
	}
	if strings.Contains(formattedStr, "main(){") {
		t.Error("autoFixFn did not run gofmt — missing space before brace in function")
	}
}

// TestAutoFixProductionWiring_AutoFixFnOnlyFormatsChangedFiles verifies that
// autoFixFn only runs gofmt/goimports on files that changed since startCommit,
// not on the entire repository.
//
// Acceptance criterion: autoFixFn uses "git diff --name-only <startCommit>" to
// identify changed .go files and only formats those files.
//
// Expected failure: No production implementation exists yet that filters changed
// files via git diff. The implementation needs to parse git diff output and
// build a file list for gofmt/goimports.
func TestAutoFixProductionWiring_AutoFixFnOnlyFormatsChangedFiles(t *testing.T) {
	// Set up a test git repository with multiple files
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	// Initialize git repo
	runCmd(t, "git", "init")
	runCmd(t, "git", "config", "user.email", "test@test.com")
	runCmd(t, "git", "config", "user.name", "Test User")

	// Create two files: one we'll modify, one we won't
	unchangedFile := filepath.Join(tmpDir, "unchanged.go")
	unchangedContent := `package main
var unchanged    =    1`
	if err := os.WriteFile(unchangedFile, []byte(unchangedContent), 0644); err != nil {
		t.Fatalf("failed to write unchanged file: %v", err)
	}

	changedFile := filepath.Join(tmpDir, "changed.go")
	changedContent := `package main
var changed    =    1`
	if err := os.WriteFile(changedFile, []byte(changedContent), 0644); err != nil {
		t.Fatalf("failed to write changed file: %v", err)
	}

	// Commit both files
	runCmd(t, "git", "add", ".")
	runCmd(t, "git", "commit", "-m", "initial commit")

	// Get the commit hash
	startCommit := strings.TrimSpace(runCmd(t, "git", "rev-parse", "HEAD"))

	// Modify only the changed file
	newChangedContent := `package main
var changed      =        2`
	if err := os.WriteFile(changedFile, []byte(newChangedContent), 0644); err != nil {
		t.Fatalf("failed to write modified file: %v", err)
	}

	// Get modification times before autofix
	unchangedStat, err := os.Stat(unchangedFile)
	if err != nil {
		t.Fatalf("failed to stat unchanged file: %v", err)
	}
	unchangedModTime := unchangedStat.ModTime()

	// Create runner and run autoFixFn
	cfg := &config.Config{
		Paths: config.PathConfig{
			Templates:       ".gromit/templates",
			Specs:           ".gromit/specs",
			Logs:            ".gromit/logs",
			ProjectClaudeMD: "CLAUDE.md",
		},
	}
	cfg.SetDefaults()

	var output strings.Builder
	r, err := NewRunner(cfg, &output)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	if r.autoFixFn == nil {
		t.Fatal("autoFixFn is nil — cannot test its behavior")
	}

	// Call autoFixFn
	if err := r.autoFixFn(startCommit); err != nil {
		t.Errorf("autoFixFn failed: %v", err)
	}

	// Verify the unchanged file was NOT modified
	unchangedStatAfter, err := os.Stat(unchangedFile)
	if err != nil {
		t.Fatalf("failed to stat unchanged file after autofix: %v", err)
	}

	if !unchangedStatAfter.ModTime().Equal(unchangedModTime) {
		unchangedContentAfter, _ := os.ReadFile(unchangedFile)
		if string(unchangedContentAfter) != unchangedContent {
			t.Error("autoFixFn modified unchanged.go — it should only format files that changed since startCommit")
		}
	}

	// Verify the changed file WAS modified (formatted)
	changedContentAfter, err := os.ReadFile(changedFile)
	if err != nil {
		t.Fatalf("failed to read changed file after autofix: %v", err)
	}

	if strings.Contains(string(changedContentAfter), "changed      =        2") {
		t.Error("autoFixFn did not format changed.go — excessive whitespace still present")
	}
}

// TestAutoFixProductionWiring_ValidationRunnerReceivesAutoFixFn verifies that
// the validation.Runner instance created in NewRunner/NewRunnerWithDeps receives
// the wired autoFixFn, not nil.
//
// Acceptance criterion: validation.NewRunner is called with r.autoFixFn as the
// third argument in both NewRunner() and NewRunnerWithDeps().
//
// Expected failure: Currently r.autoFixFn is passed to validation.NewRunner,
// but r.autoFixFn is never initialized, so validation.Runner receives nil and
// auto-fix never runs during validation recovery.
func TestAutoFixProductionWiring_ValidationRunnerReceivesAutoFixFn(t *testing.T) {
	cfg := &config.Config{
		Paths: config.PathConfig{
			Templates:       ".gromit/templates",
			Specs:           ".gromit/specs",
			Logs:            ".gromit/logs",
			ProjectClaudeMD: "CLAUDE.md",
		},
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
	}
	cfg.SetDefaults()

	var output strings.Builder
	r, err := NewRunner(cfg, &output)
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	// The validation runner should be non-nil
	if r.validationRunner == nil {
		t.Fatal("validationRunner is nil")
	}

	// We can't directly inspect validationRunner.autoFixFn (it's unexported),
	// but we can verify that r.autoFixFn is non-nil before it's passed to
	// validation.NewRunner. If r.autoFixFn is nil here, the validation runner
	// will also receive nil.
	if r.autoFixFn == nil {
		t.Error("r.autoFixFn is nil before being passed to validation.NewRunner — " +
			"this means the validation runner will not perform auto-fix during recovery")
	}
}

// runCmd is a test helper that runs a command and returns its output.
func runCmd(t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\nOutput: %s", name, args, err, out)
	}
	return string(out)
}
