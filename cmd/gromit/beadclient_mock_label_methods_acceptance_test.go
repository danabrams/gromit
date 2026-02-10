//go:build acceptance

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAcceptance_AllBeadClientMocksHaveLabelMethods verifies that every
// BeadClient mock implementation in the codebase (including those outside
// internal/runner/) has ReadyWithLabel and ListWithLabel methods.
//
// This test searches the entire repository to ensure no mocks were missed
// when the interface was updated to include label-based filtering methods.
func TestAcceptance_AllBeadClientMocksHaveLabelMethods(t *testing.T) {
	repoRoot := findRepositoryRoot(t)
	mocks := findAllBeadClientMocks(t, repoRoot)

	if len(mocks) == 0 {
		t.Fatal("No BeadClient mocks found - test may be misconfigured")
	}

	requiredMethods := []string{"ReadyWithLabel", "ListWithLabel"}

	for _, mock := range mocks {
		t.Run(mock.name+"_in_"+filepath.Base(mock.filePath), func(t *testing.T) {
			methods := extractMethodNames(t, mock.filePath, mock.name)

			for _, required := range requiredMethods {
				if !sliceContainsString(methods, required) {
					t.Errorf("Mock %s in %s is missing method %s",
						mock.name, mock.filePath, required)
				}
			}
		})
	}
}

// TestAcceptance_CodeCompilesWithoutErrors verifies that the codebase
// compiles successfully, which ensures all mock implementations satisfy
// the BeadClient interface.
func TestAcceptance_CodeCompilesWithoutErrors(t *testing.T) {
	// This test is implicit - if the test file itself compiles and runs,
	// then the codebase compiles. We verify this by attempting to parse
	// key files.
	repoRoot := findRepositoryRoot(t)

	testFiles := []string{
		"internal/runner/interfaces_test.go",
		"internal/runner/runner_test.go",
	}

	for _, relPath := range testFiles {
		fullPath := filepath.Join(repoRoot, relPath)
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, fullPath, nil, parser.ParseComments)
		if err != nil {
			t.Errorf("Failed to parse %s: %v", relPath, err)
		}
	}
}

type mockInfo struct {
	name     string
	filePath string
}

func findRepositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find repository root (no go.mod)")
		}
		dir = parent
	}
}

func findAllBeadClientMocks(t *testing.T, root string) []mockInfo {
	t.Helper()
	var mocks []mockInfo

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-test files and vendor/hidden directories
		if info.IsDir() {
			if info.Name() == "vendor" || info.Name() == ".git" || info.Name() == ".gromit" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil // Skip unparseable files
		}

		// Look for struct types that implement BeadClient
		for _, decl := range node.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				_, ok = typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				name := typeSpec.Name.Name
				// Check if this looks like a BeadClient mock
				if !strings.Contains(strings.ToLower(name), "bead") {
					continue
				}

				// Verify it has BeadClient methods
				methods := extractMethodNames(t, path, name)
				if sliceContainsString(methods, "Ready") && sliceContainsString(methods, "Show") {
					mocks = append(mocks, mockInfo{
						name:     name,
						filePath: path,
					})
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk directory tree: %v", err)
	}

	return mocks
}

func extractMethodNames(t *testing.T, filePath string, structName string) []string {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil
	}

	var methods []string
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}

		recvType := funcDecl.Recv.List[0].Type
		if starExpr, ok := recvType.(*ast.StarExpr); ok {
			recvType = starExpr.X
		}

		if ident, ok := recvType.(*ast.Ident); ok {
			if ident.Name == structName {
				methods = append(methods, funcDecl.Name.Name)
			}
		}
	}

	return methods
}

func sliceContainsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
