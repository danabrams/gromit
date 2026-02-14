//go:build acceptance

package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFinalVerificationFacadeLineCount verifies that runner.go is under 1,000
// lines after the sub-package split cleanup is complete.
//
// Acceptance criterion: runner.go contains fewer than 1,000 lines of production code.
//
// Expected failure: runner.go is currently ~2,106 lines. After cleanup it must
// be under 1,000 lines — orphaned heartbeat functions, stale helpers, and code
// that has been extracted to sub-packages must be removed from the facade.
func TestFinalVerificationFacadeLineCount(t *testing.T) {
	lines := countFileLines(t, "runner.go")
	if lines >= 1000 {
		t.Errorf("runner.go has %d lines, want < 1000 — facade needs further cleanup "+
			"(remove orphaned heartbeat functions, dead helpers, etc.)", lines)
	}
}

// TestFinalVerificationSubPackageImportIsolation verifies that sub-packages do
// not import each other (except runtypes, which is the shared types package) and
// do not import the runner/ facade package.
//
// Acceptance criterion: Sub-packages do not import each other or the runner/ facade.
//
// Expected failure: reviewpkg/reviewer.go currently imports
// "github.com/danabrams/gromit/internal/runner/escalation", which violates
// the no-sibling-import rule. After cleanup, reviewpkg should receive
// escalation behavior via a callback or the facade, not a direct import.
func TestFinalVerificationSubPackageImportIsolation(t *testing.T) {
	subPackages := []string{"runtypes", "execution", "escalation", "methodology", "validation", "reviewpkg"}
	siblingPkgs := map[string]bool{
		"execution":   true,
		"escalation":  true,
		"methodology": true,
		"validation":  true,
		"reviewpkg":   true,
	}

	for _, pkg := range subPackages {
		t.Run(pkg, func(t *testing.T) {
			pkgDir := filepath.Join(pkg)
			entries, err := os.ReadDir(pkgDir)
			if err != nil {
				t.Fatalf("cannot read sub-package directory %s: %v", pkg, err)
			}

			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
					continue
				}
				filePath := filepath.Join(pkgDir, name)
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
				if err != nil {
					t.Fatalf("failed to parse %s: %v", filePath, err)
				}

				for _, imp := range node.Imports {
					importPath := strings.Trim(imp.Path.Value, `"`)

					// Check for import of the runner facade itself
					if importPath == "github.com/danabrams/gromit/internal/runner" {
						t.Errorf("%s/%s imports the runner/ facade — sub-packages must not import their parent",
							pkg, name)
					}

					// Check for imports of sibling sub-packages (runtypes is allowed)
					for sibling := range siblingPkgs {
						if sibling == pkg {
							continue // importing yourself is fine
						}
						if strings.HasSuffix(importPath, "/runner/"+sibling) {
							t.Errorf("%s/%s imports sibling sub-package %s — "+
								"sub-packages must not import each other (use callbacks via the facade)",
								pkg, name, sibling)
						}
					}
				}
			}
		})
	}
}

// TestFinalVerificationOrphanedHeartbeatRemoved verifies that the facade no longer
// contains heartbeat functions that have been extracted to the execution/ sub-package.
// These are orphaned duplicates that must be removed during cleanup.
//
// Expected failure: runner.go currently contains startHeartbeat, startHeartbeatWithConfig,
// printHeartbeat, overwriteHeartbeat methods, plus heartbeatConfig type,
// defaultHeartbeatConfig var, and heartbeatInterval const — all of which duplicate
// execution/heartbeat.go and must be removed.
func TestFinalVerificationOrphanedHeartbeatRemoved(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "runner.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse runner.go: %v", err)
	}

	// Check for orphaned heartbeat methods on Runner
	orphanedMethods := []string{
		"startHeartbeat",
		"startHeartbeatWithConfig",
		"printHeartbeat",
		"overwriteHeartbeat",
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv == nil {
			continue
		}
		for _, method := range orphanedMethods {
			if funcDecl.Name.Name == method {
				t.Errorf("runner.go still has method %s — this heartbeat function "+
					"has been extracted to execution/heartbeat.go and should be removed from the facade",
					method)
			}
		}
	}

	// Check for orphaned heartbeat types and vars
	orphanedNames := map[string]string{
		"heartbeatConfig":        "type (extracted to execution.HeartbeatConfig)",
		"defaultHeartbeatConfig": "var (extracted to execution.DefaultHeartbeatConfig)",
		"heartbeatInterval":      "const (only used by heartbeat code now in execution/)",
	}

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok {
				if desc, found := orphanedNames[typeSpec.Name.Name]; found {
					t.Errorf("runner.go still has %s %s — should be removed from the facade",
						typeSpec.Name.Name, desc)
				}
			}
			if valueSpec, ok := spec.(*ast.ValueSpec); ok {
				for _, ident := range valueSpec.Names {
					if desc, found := orphanedNames[ident.Name]; found {
						t.Errorf("runner.go still has %s %s — should be removed from the facade",
							ident.Name, desc)
					}
				}
			}
		}
	}
}

// TestFinalVerificationReviewpkgDoesNotImportEscalation specifically verifies
// that reviewpkg/reviewer.go does not import the escalation sub-package.
// This is the known import isolation violation that must be fixed.
//
// Expected failure: reviewpkg/reviewer.go currently calls escalation.SelectTier()
// directly. After cleanup, the tier selection must be injected via a callback
// (e.g., a SelectTierFn field on the Reviewer struct) set by the facade.
func TestFinalVerificationReviewpkgDoesNotImportEscalation(t *testing.T) {
	fset := token.NewFileSet()
	reviewerPath := filepath.Join("reviewpkg", "reviewer.go")
	node, err := parser.ParseFile(fset, reviewerPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", reviewerPath, err)
	}

	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if strings.HasSuffix(importPath, "/runner/escalation") {
			t.Errorf("reviewpkg/reviewer.go imports escalation sub-package — " +
				"this violates import isolation; tier selection should be injected " +
				"via a callback (SelectTierFn) from the facade, not a direct import")
		}
	}

	// Also verify the direct call to escalation.SelectTier is gone
	node2, err := parser.ParseFile(fset, reviewerPath, nil, 0)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", reviewerPath, err)
	}

	ast.Inspect(node2, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "SelectTier" {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "escalation" {
				t.Errorf("reviewpkg/reviewer.go calls escalation.SelectTier() directly — " +
					"should use an injected SelectTierFn callback instead")
			}
		}
		return true
	})
}

// countFileLines is reused from escalation_extraction_test.go (same package).
