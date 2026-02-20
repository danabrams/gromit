package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupBeadContextCallsEscalationPolicySelectInitialTier verifies that
// setupBeadContext calls r.escalationPolicy.SelectInitialTier() rather than
// the escalation.SelectTier package helper.
//
// Expected failure: setupBeadContext (process.go:42) currently calls
// escalation.SelectTier(r.cfg, b). After implementation, it should call
// r.escalationPolicy.SelectInitialTier(b.Priority, b.Labels) instead.
func TestSetupBeadContextCallsEscalationPolicySelectInitialTier(t *testing.T) {
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
		hasEscalationSelectTier := false
		hasPolicySelectInitialTier := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			// Check for escalation.SelectTier (package-level call)
			if sel.Sel.Name == "SelectTier" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "escalation" {
					hasEscalationSelectTier = true
				}
			}
			// Check for r.escalationPolicy.SelectInitialTier (policy method call)
			if sel.Sel.Name == "SelectInitialTier" {
				if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
					if ident, ok := innerSel.X.(*ast.Ident); ok && ident.Name == "r" && innerSel.Sel.Name == "escalationPolicy" {
						hasPolicySelectInitialTier = true
					}
				}
			}
			return true
		})

		if hasEscalationSelectTier {
			t.Error("setupBeadContext still calls escalation.SelectTier() — should call r.escalationPolicy.SelectInitialTier(...) instead")
		}
		if !hasPolicySelectInitialTier {
			t.Error("setupBeadContext does not call r.escalationPolicy.SelectInitialTier() — tier selection should be delegated to the escalation policy")
		}
		return
	}
	t.Fatal("setupBeadContext not found in process.go")
}

// TestSetupBeadContextCallsEscalationSelectModel verifies that
// setupBeadContext calls escalation.SelectModel() (the package-level function)
// rather than r.escalationPolicy.SelectModel(). The package-level function
// correctly routes test-only beads to haiku, which the policy method does not.
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

		hasEscalationSelectModel := false
		hasPolicySelectModel := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "SelectModel" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "escalation" {
					hasEscalationSelectModel = true
				}
			}
			if sel.Sel.Name == "SelectModel" {
				if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
					if ident, ok := innerSel.X.(*ast.Ident); ok && ident.Name == "r" && innerSel.Sel.Name == "escalationPolicy" {
						hasPolicySelectModel = true
					}
				}
			}
			return true
		})

		if !hasEscalationSelectModel {
			t.Error("setupBeadContext does not call escalation.SelectModel() — must use the package-level function for correct test-only bead routing to haiku")
		}
		if hasPolicySelectModel {
			t.Error("setupBeadContext calls r.escalationPolicy.SelectModel() — this does not handle test-only bead routing; use escalation.SelectModel(r.cfg, b) instead")
		}
		return
	}
	t.Fatal("setupBeadContext not found in process.go")
}

// TestProcessBeadDelegatesToEscalationExecuteWithRetry verifies that the
// processBead method does not contain its own local retry loop.
// processBead may either call r.escalationHandler.ExecuteWithRetry() directly
// or delegate to r.processBeadWithContext() which itself delegates to the
// escalation handler.
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
			// Check for r.executeWithRetry (local call — forbidden)
			if sel.Sel.Name == "executeWithRetry" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
					hasLocalCall = true
				}
			}
			// Check for r.escalationHandler.ExecuteWithRetry (direct delegation)
			if sel.Sel.Name == "ExecuteWithRetry" {
				if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
					if innerSel.Sel.Name == "escalationHandler" {
						hasDelegatedCall = true
					}
				}
			}
			// Check for r.processBeadWithContext (delegation via wrapper)
			if sel.Sel.Name == "processBeadWithContext" {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "r" {
					hasDelegatedCall = true
				}
			}
			return true
		})

		if hasLocalCall {
			t.Error("processBead still calls r.executeWithRetry() locally — should delegate to r.escalationHandler.ExecuteWithRetry()")
		}
		if !hasDelegatedCall {
			t.Error("processBead does not delegate retry logic — should call r.escalationHandler.ExecuteWithRetry() or r.processBeadWithContext()")
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

// TestMakeValidationExecuteFnCallsEscalationPolicyNextTier verifies that
// makeValidationExecuteFn calls r.escalationPolicy.NextTier() rather than
// cfg.NextEscalationTier().
func TestMakeValidationExecuteFnCallsEscalationPolicyNextTier(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filepath.Join("callbacks_validation.go"), nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse callbacks_validation.go: %v", err)
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		if funcDecl.Name.Name != "makeValidationExecuteFn" {
			continue
		}

		hasCfgNextTier := false
		hasPolicyNextTier := false
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "NextEscalationTier" {
				hasCfgNextTier = true
			}
			if sel.Sel.Name == "NextTier" {
				if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
					if ident, ok := innerSel.X.(*ast.Ident); ok && ident.Name == "r" && innerSel.Sel.Name == "escalationPolicy" {
						hasPolicyNextTier = true
					}
				}
			}
			return true
		})

		if hasCfgNextTier {
			t.Error("makeValidationExecuteFn still calls cfg.NextEscalationTier() — should call r.escalationPolicy.NextTier() instead")
		}
		if !hasPolicyNextTier {
			t.Error("makeValidationExecuteFn does not call r.escalationPolicy.NextTier() — escalation tier selection should be delegated to the escalation policy")
		}
		return
	}
	t.Fatal("makeValidationExecuteFn not found in callbacks_validation.go")
}
