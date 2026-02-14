//go:build acceptance

package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestFinalVerificationOrphanedValidationSentinel verifies that the facade's
// local errValidationFailed sentinel has been removed and process.go uses the
// sub-package's exported sentinel exclusively. After the validation/ sub-package
// extraction, there should be no locally-declared sentinel in runner.go and
// process.go should return validation.ErrValidationFailed, not the local copy.
//
// Expected failure: runner.go line 36 declares `var errValidationFailed =
// errors.New("validation failed")` which duplicates validation.ErrValidationFailed.
// process.go line 335 returns this local sentinel. Both must be cleaned up.
func TestFinalVerificationOrphanedValidationSentinel(t *testing.T) {
	fset := token.NewFileSet()

	t.Run("runner.go_must_not_declare_errValidationFailed", func(t *testing.T) {
		node, err := parser.ParseFile(fset, "runner.go", nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("failed to parse runner.go: %v", err)
		}

		for _, decl := range node.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, ident := range valueSpec.Names {
					if ident.Name == "errValidationFailed" {
						t.Errorf("runner.go still declares errValidationFailed — " +
							"this duplicates validation.ErrValidationFailed and must be removed")
					}
				}
			}
		}
	})

	t.Run("process.go_must_not_return_local_errValidationFailed", func(t *testing.T) {
		node, err := parser.ParseFile(fset, "process.go", nil, 0)
		if err != nil {
			t.Fatalf("failed to parse process.go: %v", err)
		}

		// Walk AST looking for bare errValidationFailed identifiers in return
		// statements. The validation.ErrValidationFailed selector won't match
		// because the Ident "ErrValidationFailed" has a different name and sits
		// inside a SelectorExpr. Only the local facade sentinel matches as a
		// bare Ident named "errValidationFailed".
		ast.Inspect(node, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, result := range ret.Results {
				ident, ok := result.(*ast.Ident)
				if !ok {
					continue
				}
				if ident.Name == "errValidationFailed" {
					t.Errorf("process.go returns local errValidationFailed at %v — "+
						"should return validation.ErrValidationFailed instead",
						fset.Position(ident.Pos()))
				}
			}
			return true
		})
	})
}

// TestFinalVerificationReviewpkgSelectTierFnInjection verifies that reviewpkg's
// Reviewer struct uses an injected callback for tier selection instead of
// directly calling escalation.SelectTier. After cleanup, the Reviewer struct
// must have a selectTierFn field so the facade can wire in escalation.SelectTier
// without reviewpkg importing the escalation sibling package.
//
// Expected failure: reviewpkg/reviewer.go currently has SelectReviewTier as a
// standalone function that calls escalation.SelectTier(cfg, b) directly (line 103).
// After cleanup, tier selection must be injected via a selectTierFn field on the
// Reviewer struct, and the facade wires escalation.SelectTier into it.
func TestFinalVerificationReviewpkgSelectTierFnInjection(t *testing.T) {
	fset := token.NewFileSet()
	reviewerPath := filepath.Join("reviewpkg", "reviewer.go")
	node, err := parser.ParseFile(fset, reviewerPath, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", reviewerPath, err)
	}

	// Look for a selectTierFn or SelectTierFn field on the Reviewer struct,
	// indicating the callback injection pattern is in place.
	hasSelectTierFnField := false
	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if typeSpec.Name.Name != "Reviewer" {
			return true
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structType.Fields.List {
			for _, name := range field.Names {
				if name.Name == "selectTierFn" || name.Name == "SelectTierFn" {
					hasSelectTierFnField = true
				}
			}
		}
		return false
	})

	if !hasSelectTierFnField {
		t.Errorf("reviewpkg Reviewer struct does not have a selectTierFn/SelectTierFn field — " +
			"tier selection must be injected via callback to avoid importing escalation; " +
			"add a selectTierFn field and have the facade wire escalation.SelectTier into it")
	}
}
