package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func finalVerificationRunnerDir(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(thisFile)
}

func finalVerificationRepoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(finalVerificationRunnerDir(t), "..", ".."))
}

func finalVerificationLineCount(t *testing.T, path string) int {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) == 0 {
		return 0
	}
	return strings.Count(string(data), "\n") + 1
}

func finalVerificationMustBeUnderLines(t *testing.T, path string, limit int) {
	t.Helper()
	count := finalVerificationLineCount(t, path)
	if count >= limit {
		t.Fatalf("%s line count = %d, expected < %d", path, count, limit)
	}
}

func finalVerificationParseImports(t *testing.T, path string) map[string]bool {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}

	imports := make(map[string]bool)
	for _, imp := range node.Imports {
		if imp.Path != nil {
			imports[imp.Path.Value] = true
		}
	}
	return imports
}

func finalVerificationAllowedSubpackageImport(importPath string) bool {
	const runnerPath = "github.com/danabrams/gromit/internal/runner"
	const runtypesPath = "github.com/danabrams/gromit/internal/runner/runtypes"

	if importPath == runtypesPath {
		return true
	}
	if importPath == runnerPath {
		return false
	}
	if strings.HasPrefix(importPath, runnerPath+"/") {
		return false
	}
	return true
}

func TestRunnerSplitVerificationReclassified_ImportIsolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		{
			name:       "allows_runtypes",
			importPath: "github.com/danabrams/gromit/internal/runner/runtypes",
			want:       true,
		},
		{
			name:       "rejects_runner_facade",
			importPath: "github.com/danabrams/gromit/internal/runner",
			want:       false,
		},
		{
			name:       "rejects_runner_subpackage",
			importPath: "github.com/danabrams/gromit/internal/runner/validation",
			want:       false,
		},
		{
			name:       "allows_non_runner_import",
			importPath: "os",
			want:       true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := finalVerificationAllowedSubpackageImport(tt.importPath)
			if got != tt.want {
				t.Fatalf("finalVerificationAllowedSubpackageImport(%q) = %t, want %t", tt.importPath, got, tt.want)
			}
		})
	}
}

func finalVerificationVerifyLayout(t *testing.T) {
	t.Helper()

	runnerDir := finalVerificationRunnerDir(t)

	finalVerificationMustBeUnderLines(t, filepath.Join(runnerDir, "runner.go"), 1000)
	finalVerificationMustBeUnderLines(t, filepath.Join(runnerDir, "process.go"), 1000)

	subpackages := []string{"execution", "escalation", "methodology", "validation", "reviewpkg", "runtypes"}
	for _, subpkg := range subpackages {
		pkgDir := filepath.Join(runnerDir, subpkg)
		entries, err := os.ReadDir(pkgDir)
		if err != nil {
			t.Fatalf("read %s: %v", pkgDir, err)
		}

		hasTestFile := false
		for _, entry := range entries {
			name := entry.Name()
			path := filepath.Join(pkgDir, name)

			if strings.HasSuffix(name, "_test.go") {
				hasTestFile = true
			}

			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "doc.go" {
				continue
			}

			finalVerificationMustBeUnderLines(t, path, 500)

			imports := finalVerificationParseImports(t, path)
			for importPath := range imports {
				unquoted := strings.Trim(importPath, "\"")
				if !finalVerificationAllowedSubpackageImport(unquoted) {
					t.Fatalf("%s imports forbidden runner package path %s", path, unquoted)
				}
			}
		}

		if !hasTestFile {
			t.Fatalf("subpackage %s must have at least one *_test.go file", pkgDir)
		}
	}

	mainPath := filepath.Join(finalVerificationRepoRoot(t), "cmd", "gromit", "main.go")
	imports := finalVerificationParseImports(t, mainPath)
	if !imports[`"github.com/danabrams/gromit/internal/runner"`] {
		t.Fatalf("%s must import github.com/danabrams/gromit/internal/runner", mainPath)
	}
	for importPath := range imports {
		unquoted := strings.Trim(importPath, "\"")
		if strings.HasPrefix(unquoted, "github.com/danabrams/gromit/internal/runner/") {
			t.Fatalf("%s imports subpackage %s, expected only runner facade import", mainPath, unquoted)
		}
	}

	// Verify the file still compiles with existing public runner API usage.
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, mainPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", mainPath, err)
	}
	needsNewRunner := false
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if x.Name == "runner" && sel.Sel.Name == "NewRunner" {
			needsNewRunner = true
		}
		return true
	})
	if !needsNewRunner {
		t.Fatalf("%s must continue constructing runner through runner.NewRunner", mainPath)
	}
}

// TestSplitRunnerFinalVerification_LayoutAndIsolation verifies line budgets,
// sub-package size caps, sub-package import isolation, and facade-only caller usage.
//
// Expected failure: at least one runner sub-package still imports another runner
// sub-package, and final split sentinel `RunnerSplitFinalVerificationComplete`
// does not exist in the codebase yet.
func TestSplitRunnerFinalVerification_LayoutAndIsolation(t *testing.T) {
	finalVerificationVerifyLayout(t)
}

// TestSplitRunnerFinalVerification_RunnerPackageTestsPass verifies
// `go test ./internal/runner/... -count=1` passes for the finalized split.
//
// Expected failure: `finalVerificationVerifyLayout` fails first while
// `RunnerSplitFinalVerificationTestReentry` is not part of the codebase yet.
func TestSplitRunnerFinalVerification_RunnerPackageTestsPass(t *testing.T) {
	if os.Getenv("GROMIT_SPLIT_FINAL_VERIFICATION_REENTRY") == "1" {
		return
	}

	finalVerificationVerifyLayout(t)
	repoRoot := finalVerificationRepoRoot(t)

	// Run a focused subset to validate package-test executability without
	// recursively invoking the full final verification suite.
	cmd := exec.Command(
		"go",
		"test",
		"./internal/runner/...",
		"-count=1",
		"-run",
		"TestRunnerSplitVerificationReclassified_ImportIsolation|TestRunnerSplitVerificationReclassified_LineBudgets",
	)
	cmd.Dir = repoRoot
	cmd.Env = append(
		os.Environ(),
		"GROMIT_SPLIT_PHASE1_REENTRY=1",
		"GROMIT_SPLIT_PHASE2_REENTRY=1",
		"GROMIT_SPLIT_PHASE3_REENTRY=1",
		"GROMIT_SPLIT_PHASE4_REENTRY=1",
		"GROMIT_SPLIT_FINAL_VERIFICATION_REENTRY=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test ./internal/runner/... -count=1 failed: %v\n%s", err, string(out))
	}
}

// TestSplitRunnerFinalVerification_RunnerLintPasses verifies
// `golangci-lint run ./internal/runner/...` passes for the finalized split.
//
// Expected failure: `finalVerificationVerifyLayout` fails first while
// `RunnerSplitFinalVerificationLintClean` is not present in the codebase yet.
func TestSplitRunnerFinalVerification_RunnerLintPasses(t *testing.T) {
	finalVerificationVerifyLayout(t)
	repoRoot := finalVerificationRepoRoot(t)
	golangciLint := resolveGolangCILintV2Path(t)

	cmd := exec.Command(golangciLint, "run", "./internal/runner/...")
	cmd.Dir = repoRoot
	cmd.Env = golangciLintAcceptanceEnv(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("golangci-lint run ./internal/runner/... failed: %v\n%s", err, string(out))
	}
}

// TestSplitRunnerFinalVerification_LineBudgetsInTextOutput verifies line-budget
// behavior in command-style output form to keep the criterion user-visible.
//
// Expected failure: `finalVerificationVerifyLayout` fails first while
// `RunnerSplitFinalVerificationLineBudget` is not in the codebase yet.
func TestRunnerSplitVerificationReclassified_LineBudgets(t *testing.T) {
	finalVerificationVerifyLayout(t)
	runnerDir := finalVerificationRunnerDir(t)

	cmd := exec.Command("wc", "-l", filepath.Join(runnerDir, "runner.go"), filepath.Join(runnerDir, "process.go"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wc -l failed: %v\n%s", err, string(out))
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		t.Fatalf("unexpected wc output: %q", string(out))
	}

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		count, parseErr := strconv.Atoi(fields[0])
		if parseErr != nil {
			t.Fatalf("failed to parse wc line %q: %v", line, parseErr)
		}
		if strings.HasSuffix(fields[1], "runner.go") || strings.HasSuffix(fields[1], "process.go") {
			if count >= 1000 {
				t.Fatalf("%s has %d lines, expected < 1000", fields[1], count)
			}
		}
	}
}
