package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupBeadContextCallsEscalationSelectTier verifies that setupBeadContext
// calls escalation.SelectTier() rather than the Runner's local r.selectTier()
// for tier selection.
//
// Expected failure: setupBeadContext (process.go:42) currently calls r.selectTier(b),
// which is a local Runner method. After implementation, it should call
// escalation.SelectTier(r.cfg, b) instead, since the local selectTier method
// will be removed.
func TestSetupBeadContextCallsEscalationSelectTier(t *testing.T) {
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
		if funcDecl.Name.Name != "setupBeadContext" {
			continue
		}

		// Walk the AST of setupBeadContext looking for call patterns
		hasLocalSelectTier := false
		hasEscalationSelectTier := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			// Check for r.selectTier (local method call)
			if sel.Sel.Name == "selectTier" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
					hasLocalSelectTier = true
				}
			}
			// Check for escalation.SelectTier (package-level call)
			if sel.Sel.Name == "SelectTier" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "escalation" {
					hasEscalationSelectTier = true
				}
			}
			return true
		})

		if hasLocalSelectTier {
			t.Error("setupBeadContext still calls r.selectTier() — should call escalation.SelectTier(r.cfg, b) instead")
		}
		if !hasEscalationSelectTier {
			t.Error("setupBeadContext does not call escalation.SelectTier() — tier selection should be delegated to the escalation package")
		}
		return
	}
	t.Fatal("setupBeadContext not found in process.go")
}

// TestSetupBeadContextCallsEscalationSelectModel verifies that setupBeadContext
// calls escalation.SelectModel() rather than the Runner's local r.selectModel()
// for model selection.
//
// Expected failure: setupBeadContext (process.go:43) currently calls r.selectModel(b),
// which is a local Runner method. After implementation, it should call
// escalation.SelectModel(r.cfg, b) instead.
func TestSetupBeadContextCallsEscalationSelectModel(t *testing.T) {
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
		if funcDecl.Name.Name != "setupBeadContext" {
			continue
		}

		hasLocalSelectModel := false
		hasEscalationSelectModel := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "selectModel" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
					hasLocalSelectModel = true
				}
			}
			if sel.Sel.Name == "SelectModel" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "escalation" {
					hasEscalationSelectModel = true
				}
			}
			return true
		})

		if hasLocalSelectModel {
			t.Error("setupBeadContext still calls r.selectModel() — should call escalation.SelectModel(r.cfg, b) instead")
		}
		if !hasEscalationSelectModel {
			t.Error("setupBeadContext does not call escalation.SelectModel() — model selection should be delegated to the escalation package")
		}
		return
	}
	t.Fatal("setupBeadContext not found in process.go")
}

// TestProcessBeadDelegatesToEscalationExecuteWithRetry verifies that the
// processBead method delegates to r.escalationHandler.ExecuteWithRetry()
// rather than calling the Runner's local executeWithRetry method.
//
// Expected failure: processBead currently calls r.executeWithRetry (runner.go:880)
// which is a local method with its own ~100-line retry loop. After implementation,
// processBead should call r.escalationHandler.ExecuteWithRetry() with an InvokeFn
// callback wrapping execution.Invoker.Execute.
func TestProcessBeadDelegatesToEscalationExecuteWithRetry(t *testing.T) {
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
		if funcDecl.Name.Name != "processBead" {
			continue
		}

		// Scan the function body source for patterns indicating delegation
		hasLocalCall := false
		hasDelegatedCall := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			// Check for r.executeWithRetry (local call)
			if sel.Sel.Name == "executeWithRetry" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
					hasLocalCall = true
				}
			}
			// Check for r.escalationHandler.ExecuteWithRetry (delegated call)
			if sel.Sel.Name == "ExecuteWithRetry" {
				// The receiver should be r.escalationHandler
				if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
					if innerSel.Sel.Name == "escalationHandler" {
						hasDelegatedCall = true
					}
				}
			}
			return true
		})

		if hasLocalCall {
			t.Error("processBead still calls r.executeWithRetry() locally — should delegate to r.escalationHandler.ExecuteWithRetry()")
		}
		if !hasDelegatedCall {
			t.Error("processBead does not call r.escalationHandler.ExecuteWithRetry() — retry logic should be delegated to the escalation handler")
		}
		return
	}
	t.Fatal("processBead not found in runner.go")
}

// TestProcessGoUsesEscalationLearningFunctions verifies that process.go calls
// the escalation package's learning extraction functions rather than having
// local extractLearning/extractSyntheticLearning methods.
//
// Expected failure: process.go currently has local methods like r.extractLearning(),
// r.extractSyntheticLearning(), r.extractScopeTooLargeLearning(), etc.
// After implementation, callers should use escalation.ExtractLearning(),
// escalation.ExtractSyntheticLearning(), etc.
func TestProcessGoUsesEscalationLearningFunctions(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("process.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse process.go: %v", err)
	}

	localLearningFns := map[string]bool{
		"extractLearning":              false,
		"extractSyntheticLearning":     false,
		"extractScopeTooLargeLearning": false,
		"extractTimeoutLearning":       false,
		"extractSuccessLearning":       false,
	}

	// Check all Runner methods for calls to local extract* methods
	localCalls := []string{}
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		// Skip the learning methods themselves
		if _, isLearningFn := localLearningFns[funcDecl.Name.Name]; isLearningFn {
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
			if _, isLearning := localLearningFns[sel.Sel.Name]; isLearning {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
					localCalls = append(localCalls,
						funcDecl.Name.Name+"() calls r."+sel.Sel.Name+"()")
				}
			}
			return true
		})
	}

	if len(localCalls) > 0 {
		t.Errorf("process.go still calls local learning extraction methods — "+
			"these should be replaced with escalation.ExtractLearning/etc:\n  %s",
			strings.Join(localCalls, "\n  "))
	}
}
