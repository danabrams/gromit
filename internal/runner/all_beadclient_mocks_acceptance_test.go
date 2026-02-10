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

// TestAcceptance_AllBeadClientMocksHaveLabelMethods verifies that every mock
// implementation of the BeadClient interface in the codebase has ReadyWithLabel
// and ListWithLabel methods. This test searches the entire codebase for mock
// implementations to ensure none are missed.
func TestAcceptance_AllBeadClientMocksHaveLabelMethods(t *testing.T) {
	repoRoot := findRepoRoot(t)
	mockStructs := findBeadClientMockStructs(t, repoRoot)

	if len(mockStructs) == 0 {
		t.Fatal("No BeadClient mock structs found in codebase - test may be broken")
	}

	// Expected methods that all mocks must have
	requiredMethods := []string{"ReadyWithLabel", "ListWithLabel"}

	for _, mock := range mockStructs {
		t.Run(mock.Name, func(t *testing.T) {
			methods := findMethodsForStruct(t, mock.FilePath, mock.Name)

			for _, reqMethod := range requiredMethods {
				found := false
				for _, method := range methods {
					if method == reqMethod {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Mock struct %s in %s is missing required method %s",
						mock.Name, mock.FilePath, reqMethod)
				}
			}

			// Verify it has the basic BeadClient methods too
			baselineMethods := []string{"Ready", "Show", "Close", "Sync"}
			for _, baseMethod := range baselineMethods {
				found := false
				for _, method := range methods {
					if method == baseMethod {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Mock struct %s in %s is missing baseline method %s",
						mock.Name, mock.FilePath, baseMethod)
				}
			}
		})
	}
}

// TestAcceptance_NoBeadClientMocksExistOutsideInternalRunner verifies that
// all BeadClient mock implementations are in internal/runner/, as per project
// conventions. This ensures we don't have scattered mock implementations.
func TestAcceptance_NoBeadClientMocksExistOutsideInternalRunner(t *testing.T) {
	repoRoot := findRepoRoot(t)
	mockStructs := findBeadClientMockStructs(t, repoRoot)

	if len(mockStructs) == 0 {
		t.Fatal("No BeadClient mock structs found - expected at least mockBeadClient and mockBeadClientForStatus")
	}

	for _, mock := range mockStructs {
		relPath, err := filepath.Rel(repoRoot, mock.FilePath)
		if err != nil {
			t.Fatalf("Failed to get relative path for %s: %v", mock.FilePath, err)
		}

		if !strings.HasPrefix(relPath, "internal/runner/") {
			t.Errorf("Found BeadClient mock %s outside internal/runner/ at: %s\n"+
				"All BeadClient mocks should be in internal/runner/ for consistency",
				mock.Name, relPath)
		}
	}
}

// TestAcceptance_AllKnownMocksAreDetected verifies that the search algorithm
// finds the mocks we know exist (mockBeadClient and mockBeadClientForStatus).
func TestAcceptance_AllKnownMocksAreDetected(t *testing.T) {
	repoRoot := findRepoRoot(t)
	mockStructs := findBeadClientMockStructs(t, repoRoot)

	knownMocks := map[string]bool{
		"mockBeadClient":          false,
		"mockBeadClientForStatus": false,
	}

	for _, mock := range mockStructs {
		if _, exists := knownMocks[mock.Name]; exists {
			knownMocks[mock.Name] = true
		}
	}

	for name, found := range knownMocks {
		if !found {
			t.Errorf("Known mock %s was not detected by the search algorithm", name)
		}
	}
}

// mockStructInfo holds information about a found mock struct
type mockStructInfo struct {
	Name     string
	FilePath string
}

// findRepoRoot finds the repository root by looking for go.mod
func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up the directory tree looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find repository root (no go.mod found)")
		}
		dir = parent
	}
}

// findBeadClientMockStructs searches the codebase for struct types that
// implement the BeadClient interface (by having a Ready() method)
func findBeadClientMockStructs(t *testing.T, root string) []mockStructInfo {
	t.Helper()

	var mocks []mockStructInfo

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and .git directories
		if info.IsDir() && (info.Name() == "vendor" || info.Name() == ".git" || info.Name() == ".gromit") {
			return filepath.SkipDir
		}

		// Only process test files
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse the file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			// Skip files that can't be parsed
			return nil
		}

		// Look for struct types with "mock" in the name
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

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				// Check if it looks like a BeadClient mock
				name := typeSpec.Name.Name
				if !strings.Contains(strings.ToLower(name), "bead") {
					continue
				}

				if !strings.Contains(strings.ToLower(name), "client") && !strings.Contains(strings.ToLower(name), "mock") {
					continue
				}

				// Check if it has methods that match BeadClient interface
				methods := findMethodsForStruct(t, path, name)
				hasReady := false
				hasShow := false
				for _, method := range methods {
					if method == "Ready" {
						hasReady = true
					}
					if method == "Show" {
						hasShow = true
					}
				}

				if hasReady && hasShow {
					mocks = append(mocks, mockStructInfo{
						Name:     name,
						FilePath: path,
					})
				}

				// Also check struct fields for function pointers (FnField pattern)
				for _, field := range structType.Fields.List {
					if len(field.Names) > 0 {
						fieldName := field.Names[0].Name
						if strings.Contains(fieldName, "ReadyFn") || strings.Contains(fieldName, "ShowFn") {
							// This looks like a mock with function pointer fields
							mocks = append(mocks, mockStructInfo{
								Name:     name,
								FilePath: path,
							})
							break
						}
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk directory tree: %v", err)
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []mockStructInfo
	for _, mock := range mocks {
		key := mock.FilePath + ":" + mock.Name
		if !seen[key] {
			seen[key] = true
			unique = append(unique, mock)
		}
	}

	return unique
}

// findMethodsForStruct finds all methods defined for a given struct type
func findMethodsForStruct(t *testing.T, filePath string, structName string) []string {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		t.Logf("Failed to parse %s: %v", filePath, err)
		return nil
	}

	var methods []string

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}

		// Get the receiver type
		recvType := funcDecl.Recv.List[0].Type

		// Handle pointer receivers
		if starExpr, ok := recvType.(*ast.StarExpr); ok {
			recvType = starExpr.X
		}

		// Check if it matches our struct
		if ident, ok := recvType.(*ast.Ident); ok {
			if ident.Name == structName {
				methods = append(methods, funcDecl.Name.Name)
			}
		}
	}

	return methods
}

// TestAcceptance_MocksCompileSatisfyInterface is a compile-time check that
// ensures all known mocks satisfy the BeadClient interface
func TestAcceptance_MocksCompileSatisfyInterface(t *testing.T) {
	// This is a compile-time check - if these assignments fail to compile,
	// then the mocks don't satisfy the interface
	var _ BeadClient = (*mockBeadClient)(nil)
	var _ BeadClient = (*mockBeadClientForStatus)(nil)

	// If we get here, the test passes (compilation succeeded)
	t.Log("All mocks successfully satisfy BeadClient interface")
}
