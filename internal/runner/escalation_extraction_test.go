package runner

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// TestProcessGoLocalEscalationMethodsRemoved verifies that the local
// escalation methods in process.go have been removed after extraction
// into the escalation/ sub-package.
//
// Expected failure: process.go still contains local implementations of
// handleStallTimeout, analyzeAndHandleFailure, handleEscalation, escalateTier,
// attemptDecomposition, extractLearning, extractSyntheticLearning,
// extractScopeTooLargeLearning, extractTimeoutLearning, extractSuccessLearning.
// These should be removed once the Runner delegates to escalation.Handler.
func TestProcessGoLocalEscalationMethodsRemoved(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("process.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse process.go: %v", err)
	}

	// These methods should no longer exist in process.go — they have been
	// extracted to the escalation/ sub-package.
	removedMethods := map[string]string{
		"handleStallTimeout":          "escalation.Handler.HandleStallTimeout",
		"analyzeAndHandleFailure":     "escalation.Handler.AnalyzeAndHandleFailure",
		"handleEscalation":            "escalation.Handler.HandleEscalation",
		"escalateTier":                "escalation.Handler.EscalateTier",
		"attemptDecomposition":        "escalation.Handler.AttemptDecomposition",
		"extractLearning":             "escalation.ExtractLearning",
		"extractSyntheticLearning":    "escalation.ExtractSyntheticLearning",
		"extractScopeTooLargeLearning": "escalation.ExtractScopeTooLargeLearning",
		"extractTimeoutLearning":      "escalation.ExtractTimeoutLearning",
		"extractSuccessLearning":      "escalation.ExtractSuccessLearning",
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		// Only check methods on *Runner
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		name := funcDecl.Name.Name
		if replacement, found := removedMethods[name]; found {
			t.Errorf("process.go still contains Runner.%s() — this should be removed and delegated to %s",
				name, replacement)
		}
	}
}

// TestRunnerGoLocalEscalationMethodsRemoved verifies that selectTier and
// selectModel methods on Runner have been removed from runner.go after
// extraction into the escalation/ sub-package.
//
// Expected failure: runner.go still contains Runner.selectTier (line 982) and
// Runner.selectModel (line 1001). These should be replaced by calls to
// escalation.SelectTier() and escalation.SelectModel().
func TestRunnerGoLocalEscalationMethodsRemoved(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("runner.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse runner.go: %v", err)
	}

	removedMethods := map[string]string{
		"selectTier":  "escalation.SelectTier",
		"selectModel": "escalation.SelectModel",
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		name := funcDecl.Name.Name
		if replacement, found := removedMethods[name]; found {
			t.Errorf("runner.go still contains Runner.%s() — this should be removed and delegated to %s",
				name, replacement)
		}
	}
}

// TestRunnerGoExecuteWithRetryRemoved verifies that the local executeWithRetry
// method on Runner in runner.go has been removed (replaced by delegation to
// escalation.Handler.ExecuteWithRetry via the InvokeFn callback pattern).
//
// Expected failure: runner.go still contains Runner.executeWithRetry (line 880).
// After implementation, the Runner should delegate to r.escalationHandler.ExecuteWithRetry()
// with an InvokeFn callback that wraps execution.Invoker.Execute.
func TestRunnerGoExecuteWithRetryRemoved(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("runner.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse runner.go: %v", err)
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		if funcDecl.Name.Name == "executeWithRetry" {
			t.Errorf("runner.go still contains Runner.executeWithRetry() — this ~100-line method should be replaced by a thin wrapper that delegates to r.escalationHandler.ExecuteWithRetry()")
		}
	}
}

// TestProcessGoEscalateModelRemoved verifies that the local escalateModel
// method has been removed from process.go.
//
// Expected failure: process.go still contains Runner.escalateModel (line 503).
// This method updates model name directly; after extraction, escalation.Handler.EscalateTier
// handles both tier and model updates.
func TestProcessGoEscalateModelRemoved(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("process.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse process.go: %v", err)
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		if funcDecl.Name.Name == "escalateModel" {
			t.Errorf("process.go still contains Runner.escalateModel() — this should be removed; escalation.Handler.EscalateTier handles model name updates")
		}
	}
}

// countFileLines counts total lines in a file.
func countFileLines(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open %s: %v", path, err)
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan %s: %v", path, err)
	}
	return count
}

// TestRunnerGoUnder1000Lines verifies that runner.go contains fewer than 1,000
// total lines after extraction of escalation logic.
//
// Expected failure: runner.go currently has ~2,317 lines. After extracting
// executeWithRetry (~100 lines), selectTier, selectModel, and related
// escalation code, it should be under 1,000 lines total.
func TestRunnerGoUnder1000Lines(t *testing.T) {
	path := filepath.Join("runner.go")
	lines := countFileLines(t, path)
	const limit = 1000
	if lines >= limit {
		t.Errorf("runner.go has %d lines, want < %d — extraction of escalation logic should reduce this", lines, limit)
	}
}

// TestProcessGoUnder1000Lines verifies that process.go contains fewer than
// 1,000 total lines after extraction of escalation logic.
//
// Expected failure: process.go currently has ~1,308 lines. After removing
// handleStallTimeout, analyzeAndHandleFailure, handleEscalation, escalateTier,
// attemptDecomposition, escalateModel, and all extractLearning variants
// (~500+ lines), it should be under 1,000 lines total.
func TestProcessGoUnder1000Lines(t *testing.T) {
	path := filepath.Join("process.go")
	lines := countFileLines(t, path)
	const limit = 1000
	if lines >= limit {
		t.Errorf("process.go has %d lines, want < %d — extraction of escalation logic should reduce this", lines, limit)
	}
}

