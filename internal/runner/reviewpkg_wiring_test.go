package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// --- Runner struct has reviewer field ---

// TestRunnerStructHasReviewerField uses AST parsing to verify that the Runner
// struct definition in runner.go includes a field named "reviewer" of type
// *reviewpkg.Reviewer.
func TestRunnerStructHasReviewerField(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("runner.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse runner.go: %v", err)
	}

	found := false
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "Runner" {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					if name.Name == "reviewer" {
						found = true
						// Verify the type is *reviewpkg.Reviewer
						starExpr, ok := field.Type.(*ast.StarExpr)
						if !ok {
							t.Error("reviewer field should be a pointer type (*reviewpkg.Reviewer)")
							return
						}
						selExpr, ok := starExpr.X.(*ast.SelectorExpr)
						if !ok {
							t.Error("reviewer field type should be a selector (reviewpkg.Reviewer)")
							return
						}
						if ident, ok := selExpr.X.(*ast.Ident); ok {
							if ident.Name != "reviewpkg" {
								t.Errorf("reviewer field package = %q, want %q", ident.Name, "reviewpkg")
							}
						}
						if selExpr.Sel.Name != "Reviewer" {
							t.Errorf("reviewer field type = %q, want %q", selExpr.Sel.Name, "Reviewer")
						}
					}
				}
			}
		}
	}

	if !found {
		t.Error("Runner struct in runner.go does not have a 'reviewer' field — " +
			"add reviewer *reviewpkg.Reviewer to the Runner struct")
	}
}

// --- Constructor wiring ---

// TestNewRunnerWithDepsWiresReviewer verifies that newRunnerWithDepsImpl creates
// and assigns a reviewpkg.Reviewer to the Runner's reviewer field.
// Uses AST to check that the constructor body contains reviewer assignment.
func TestNewRunnerWithDepsWiresReviewer(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("constructor_with_deps.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse constructor_with_deps.go: %v", err)
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Name.Name != "newRunnerWithDepsImpl" {
			continue
		}

		// Look for either:
		// 1. r.reviewer = ... (assignment after struct init)
		// 2. reviewer: ... in the struct literal
		hasReviewerAssignment := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			// Check for r.reviewer = <something> assignment
			assign, ok := n.(*ast.AssignStmt)
			if ok {
				for _, lhs := range assign.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if ok && sel.Sel.Name == "reviewer" {
						hasReviewerAssignment = true
					}
				}
			}
			// Check for reviewer: <value> in struct literal (composite lit key-value)
			kv, ok := n.(*ast.KeyValueExpr)
			if ok {
				ident, ok := kv.Key.(*ast.Ident)
				if ok && ident.Name == "reviewer" {
					hasReviewerAssignment = true
				}
			}
			return true
		})

		if !hasReviewerAssignment {
			t.Error("newRunnerWithDepsImpl does not assign the reviewer field — " +
				"should create a reviewpkg.Reviewer and assign it to r.reviewer")
		}
		return
	}
	t.Fatal("newRunnerWithDepsImpl not found in constructor_with_deps.go")
}

// TestNewRunnerWiresReviewer verifies that the production NewRunner constructor
// returns an Orchestrator with a properly wired Review stage for code review.
func TestNewRunnerWiresReviewer(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("constructor.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse constructor.go: %v", err)
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Name.Name != "newRunnerImpl" {
			continue
		}

		hasReviewStageCreation := false
		hasReviewStageInConfig := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			// Check for review.New(...) call to create the Review stage
			call, ok := n.(*ast.CallExpr)
			if ok {
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == "New" {
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "review" {
						hasReviewStageCreation = true
					}
				}
			}
			// Check for Review field assignment in OrchestratorConfig struct literal
			kv, ok := n.(*ast.KeyValueExpr)
			if ok {
				ident, ok := kv.Key.(*ast.Ident)
				if ok && ident.Name == "Review" {
					hasReviewStageInConfig = true
				}
			}
			return true
		})

		if !hasReviewStageCreation {
			t.Error("newRunnerImpl does not call review.New() — " +
				"should create a review.Stage for LLM code review")
		}
		if !hasReviewStageInConfig {
			t.Error("newRunnerImpl does not assign Review stage to OrchestratorConfig — " +
				"Review stage should be part of the orchestrator configuration")
		}
		return
	}
	t.Fatal("newRunnerImpl not found in constructor.go")
}

// --- Local review methods removed from runner.go ---

// TestRunnerGoLocalReviewMethodsRemoved verifies that the Runner's local review
// methods have been removed from runner.go in favor of delegation to r.reviewer.
func TestRunnerGoLocalReviewMethodsRemoved(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("runner.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse runner.go: %v", err)
	}

	removedMethods := map[string]string{
		"runLightReview":    "reviewer.RunLight",
		"applyReviewResult": "reviewer.ApplyResult",
		"writeReviewLog":    "reviewer.WriteReviewLog",
		"runThoroughReview": "reviewer.RunThorough",
		"selectReviewTier":  "reviewpkg.SelectReviewTier",
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		if replacement, found := removedMethods[funcDecl.Name.Name]; found {
			t.Errorf("runner.go still has func (r *Runner) %s — should be deleted, "+
				"replaced by delegation to r.%s", funcDecl.Name.Name, replacement)
		}
	}
}

// TestRunnerGoLocalReviewFunctionsRemoved verifies that package-level review
// helper functions have been removed from runner.go (they live in reviewpkg now).
func TestRunnerGoLocalReviewFunctionsRemoved(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("runner.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse runner.go: %v", err)
	}

	removedFunctions := []string{
		"selectReviewModel",
		"buildReviewBeadLabels",
		"buildBacklogLabels",
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv != nil {
			continue // skip methods
		}
		for _, fnName := range removedFunctions {
			if funcDecl.Name.Name == fnName {
				t.Errorf("runner.go still has func %s — this function should be "+
					"deleted after moving to reviewpkg", fnName)
			}
		}
	}
}

// TestProcessGoLocalRunPostSuccessReviewRemoved verifies that the Runner's local
// runPostSuccessReview method has been removed from process.go.
func TestProcessGoLocalRunPostSuccessReviewRemoved(t *testing.T) {
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
		if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
			if funcDecl.Name.Name == "runPostSuccessReview" {
				t.Error("process.go still has func (r *Runner) runPostSuccessReview — " +
					"this method should be deleted, replaced by r.reviewer.RunPostSuccess")
			}
		}
	}
}

// --- Delegation in processBead and Run ---

// TestHandleValidationResultDelegatesToReviewerRunPostSuccess verifies that
// handleValidationResult calls r.reviewer.RunPostSuccess (delegated) rather
// than r.runPostSuccessReview (local). handleValidationResult in process.go
// is where the post-success review call originates.
func TestHandleValidationResultDelegatesToReviewerRunPostSuccess(t *testing.T) {
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
		if funcDecl.Name.Name != "handleValidationResult" {
			continue
		}

		hasLocalReviewCall := false
		hasReviewerDelegation := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			// Check for r.runPostSuccessReview (local method call)
			if sel.Sel.Name == "runPostSuccessReview" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
					hasLocalReviewCall = true
				}
			}
			// Check for r.reviewer.RunPostSuccess (delegated call)
			if sel.Sel.Name == "RunPostSuccess" {
				if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
					if innerSel.Sel.Name == "reviewer" {
						hasReviewerDelegation = true
					}
				}
			}
			return true
		})

		if hasLocalReviewCall {
			t.Error("handleValidationResult still calls r.runPostSuccessReview() locally — " +
				"should delegate to r.reviewer.RunPostSuccess()")
		}
		if !hasReviewerDelegation {
			t.Error("handleValidationResult does not call r.reviewer.RunPostSuccess() — " +
				"post-success review should be delegated to the reviewer")
		}
		return
	}
	t.Fatal("handleValidationResult not found in process.go")
}

// TestRunDelegatesToReviewerRunThorough verifies that the Run method calls
// r.reviewer.RunThorough instead of r.runThoroughReview for thorough reviews.
func TestRunDelegatesToReviewerRunThorough(t *testing.T) {
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
		if funcDecl.Name.Name != "Run" {
			continue
		}
		// Ensure we're looking at func (r *Runner) Run, not other types' Run methods
		recvType := funcDecl.Recv.List[0].Type
		if starExpr, ok := recvType.(*ast.StarExpr); ok {
			if ident, ok := starExpr.X.(*ast.Ident); ok && ident.Name != "Runner" {
				continue
			}
		}

		hasLocalThoroughCall := false
		hasReviewerDelegation := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			// Check for r.runThoroughReview (local method call)
			if sel.Sel.Name == "runThoroughReview" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
					hasLocalThoroughCall = true
				}
			}
			// Check for r.reviewer.RunThorough (delegated call)
			if sel.Sel.Name == "RunThorough" {
				if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
					if innerSel.Sel.Name == "reviewer" {
						hasReviewerDelegation = true
					}
				}
			}
			return true
		})

		if hasLocalThoroughCall {
			t.Error("Run() still calls r.runThoroughReview() locally — " +
				"should delegate to r.reviewer.RunThorough()")
		}
		if !hasReviewerDelegation {
			t.Error("Run() does not call r.reviewer.RunThorough() — " +
				"thorough review should be delegated to the reviewer")
		}
		return
	}
	t.Fatal("Run not found in runner.go")
}

// TestReviewerUsesPhaseFilteredRulesForReviewInvocations verifies reviewpkg's
// RunLight and RunThorough load rules through LoadRulesForPhase("review")
// rather than LoadRules().
func TestReviewerUsesPhaseFilteredRulesForReviewInvocations(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("reviewpkg", "reviewer.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse reviewpkg/reviewer.go: %v", err)
	}

	targets := map[string]bool{
		"RunLight":    true,
		"RunThorough": true,
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		if !targets[funcDecl.Name.Name] {
			continue
		}

		var hasLoadRules bool
		var hasPhaseLoadReview bool
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "LoadRules":
				hasLoadRules = true
			case "LoadRulesForPhase":
				if len(call.Args) != 1 {
					return true
				}
				lit, ok := call.Args[0].(*ast.Ident)
				if ok && lit.Name == "reviewPhase" {
					hasPhaseLoadReview = true
				}
			}
			return true
		})

		if hasLoadRules {
			t.Errorf("%s calls LoadRules(); expected LoadRulesForPhase(reviewPhase)", funcDecl.Name.Name)
		}
		if !hasPhaseLoadReview {
			t.Errorf("%s does not call LoadRulesForPhase(reviewPhase)", funcDecl.Name.Name)
		}
		delete(targets, funcDecl.Name.Name)
	}

	for missing := range targets {
		t.Errorf("missing expected method in reviewer.go: %s", missing)
	}
}

// --- process.go does not call removed local review methods ---

// TestProcessGoDoesNotCallLocalReviewMethods verifies that process.go does not
// call any local review methods that should have been replaced by reviewer delegation.
func TestProcessGoDoesNotCallLocalReviewMethods(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("process.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse process.go: %v", err)
	}

	removedCalls := []string{
		"runLightReview",
		"applyReviewResult",
		"writeReviewLog",
	}

	var violations []string
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			for _, removed := range removedCalls {
				if sel.Sel.Name == removed {
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
						violations = append(violations,
							funcDecl.Name.Name+"() calls r."+removed+"()")
					}
				}
			}
			return true
		})
	}

	if len(violations) > 0 {
		t.Errorf("process.go still calls local review methods that should be "+
			"replaced with r.reviewer delegation:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// --- runner.go imports reviewpkg ---

// TestRunnerGoImportsReviewpkg verifies that runner.go imports the reviewpkg
// package, which is required for the reviewer field type declaration.
func TestRunnerGoImportsReviewpkg(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("runner.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse runner.go: %v", err)
	}

	hasReviewpkgImport := false
	for _, imp := range node.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.HasSuffix(path, "/runner/reviewpkg") {
			hasReviewpkgImport = true
			break
		}
	}

	if !hasReviewpkgImport {
		t.Error("runner.go does not import the reviewpkg package — " +
			"required for the reviewer *reviewpkg.Reviewer field")
	}
}

// --- Old test functions calling removed methods must be cleaned up ---

// TestOldReviewTestFunctionsRemovedFromRunnerTest verifies that test functions
// in runner_test.go that call the removed local review methods have been deleted.
// These tests called selectReviewModel, buildReviewBeadLabels, buildBacklogLabels,
// r.applyReviewResult, r.runLightReview, and r.runThoroughReview — all of which
// have moved to reviewpkg. Keeping them causes compilation failures.
func TestOldReviewTestFunctionsRemovedFromRunnerTest(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("runner_test.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse runner_test.go: %v", err)
	}

	// These test functions call removed local methods and must be deleted
	removedTests := map[string]string{
		"TestSelectReviewModel":                          "selectReviewModel (moved to reviewpkg)",
		"TestSelectReviewModelNilConfig":                 "selectReviewModel (moved to reviewpkg)",
		"TestBuildReviewBeadLabels":                      "buildReviewBeadLabels (moved to reviewpkg)",
		"TestBuildBacklogLabels":                         "buildBacklogLabels (moved to reviewpkg)",
		"TestApplyReviewResultNilRunner":                 "r.applyReviewResult (moved to reviewpkg.ApplyResult)",
		"TestApplyReviewResultNilResult":                 "r.applyReviewResult (moved to reviewpkg.ApplyResult)",
		"TestApplyReviewResultNilBeads":                  "r.applyReviewResult (moved to reviewpkg.ApplyResult)",
		"TestApplyReviewResultCreatesBeads":              "r.applyReviewResult (moved to reviewpkg.ApplyResult)",
		"TestApplyReviewResultCreatesBacklogItems":       "r.applyReviewResult (moved to reviewpkg.ApplyResult)",
		"TestApplyReviewResultHandlesCreateErrors":       "r.applyReviewResult (moved to reviewpkg.ApplyResult)",
		"TestRunLightReviewSkipsWhenDeadlineExpired":     "r.runLightReview (moved to reviewpkg.RunLight)",
		"TestRunLightReviewSkipsWhenInsufficientTime":    "r.runLightReview (moved to reviewpkg.RunLight)",
		"TestRunThoroughReviewSkipsWhenDeadlineExpired":  "r.runThoroughReview (moved to reviewpkg.RunThorough)",
		"TestRunThoroughReviewSkipsWhenInsufficientTime": "r.runThoroughReview (moved to reviewpkg.RunThorough)",
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if reason, found := removedTests[funcDecl.Name.Name]; found {
			t.Errorf("runner_test.go still contains %s — this test calls %s and must be deleted",
				funcDecl.Name.Name, reason)
		}
	}
}

// TestOldSelectReviewTierTestFileRemovedOrUpdated verifies that
// select_review_tier_test.go no longer calls the removed Runner method
// r.selectReviewTier. The selectReviewTier method has moved to
// reviewpkg.SelectReviewTier.
func TestOldSelectReviewTierTestFileRemovedOrUpdated(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("select_review_tier_test.go"), nil, parser.ParseComments)
	if err != nil {
		// File was deleted — that's valid
		return
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Body == nil {
			continue
		}

		// Check if any function calls r.selectReviewTier (removed method)
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "selectReviewTier" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
					t.Errorf("select_review_tier_test.go function %s calls r.selectReviewTier — "+
						"this method has moved to reviewpkg.SelectReviewTier",
						funcDecl.Name.Name)
				}
			}
			return true
		})
	}
}
