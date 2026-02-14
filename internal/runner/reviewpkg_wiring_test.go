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
//
// Expected failure: Runner struct in runner.go does not have a reviewer field yet.
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

// TestNewRunnerWithDepsWiresReviewer verifies that NewRunnerWithDeps creates
// and assigns a reviewpkg.Reviewer to the Runner's reviewer field.
// Uses AST to check that the constructor body contains reviewer assignment.
//
// Expected failure: NewRunnerWithDeps does not assign a reviewer field yet.
func TestNewRunnerWithDepsWiresReviewer(t *testing.T) {
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
		if funcDecl.Name.Name != "NewRunnerWithDeps" {
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
			t.Error("NewRunnerWithDeps does not assign the reviewer field — " +
				"should create a reviewpkg.Reviewer and assign it to r.reviewer")
		}
		return
	}
	t.Fatal("NewRunnerWithDeps not found in runner.go")
}

// TestNewRunnerWiresReviewer verifies that the production NewRunner constructor
// also creates and assigns a reviewer field.
//
// Expected failure: NewRunner does not assign a reviewer field yet.
func TestNewRunnerWiresReviewer(t *testing.T) {
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
		if funcDecl.Name.Name != "NewRunner" {
			continue
		}
		// Skip methods (NewRunner is package-level)
		if funcDecl.Recv != nil {
			continue
		}

		hasReviewerAssignment := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if ok {
				for _, lhs := range assign.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if ok && sel.Sel.Name == "reviewer" {
						hasReviewerAssignment = true
					}
				}
			}
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
			t.Error("NewRunner does not assign the reviewer field — " +
				"should create a reviewpkg.Reviewer and assign it to r.reviewer")
		}
		return
	}
	t.Fatal("NewRunner not found in runner.go")
}

// --- Local review methods removed from runner.go ---

// TestRunnerGoLocalReviewMethodsRemoved verifies that the Runner's local review
// methods have been removed from runner.go in favor of delegation to r.reviewer.
//
// Expected failure: runner.go still contains these methods.
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
//
// Expected failure: runner.go still contains these functions.
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
//
// Expected failure: process.go still contains func (r *Runner) runPostSuccessReview(...).
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
//
// Expected failure: handleValidationResult currently calls r.runPostSuccessReview.
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
//
// Expected failure: Run() calls r.runThoroughReview (local method in runner.go).
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

// --- process.go does not call removed local review methods ---

// TestProcessGoDoesNotCallLocalReviewMethods verifies that process.go does not
// call any local review methods that should have been replaced by reviewer delegation.
//
// Expected failure: process.go calls r.runLightReview, r.applyReviewResult,
// r.writeReviewLog, and selectReviewModel.
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
//
// Expected failure: runner.go does not import reviewpkg yet.
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
