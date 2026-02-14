package runner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestClaudeClientInterfaceRemoved verifies that the ClaudeClient interface
// has been removed from internal/runner/interfaces.go.
//
// Expected failure: ClaudeClient interface still exists in interfaces.go
func TestClaudeClientInterfaceRemoved(t *testing.T) {
	interfacesPath := filepath.Join("interfaces.go")

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, interfacesPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse interfaces.go: %v", err)
	}

	// Check that ClaudeClient interface is not defined
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if typeSpec.Name.Name == "ClaudeClient" {
				t.Errorf("ClaudeClient interface still exists in interfaces.go - it should be removed because Runner no longer uses this interface (all LLM access goes through Provider/Router)")
			}
		}
	}
}

// TestClaudeClientCompileTimeCheckRemoved verifies that the compile-time
// assertion for ClaudeClient has been removed from internal/runner/interfaces.go.
//
// Expected failure: compile-time check "var _ ClaudeClient = (*claude.Client)(nil)" still exists
func TestClaudeClientCompileTimeCheckRemoved(t *testing.T) {
	interfacesPath := filepath.Join("interfaces.go")

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, interfacesPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse interfaces.go: %v", err)
	}

	// Look for the compile-time check in variable declarations
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			// Check if the type is ClaudeClient
			if ident, ok := valueSpec.Type.(*ast.Ident); ok && ident.Name == "ClaudeClient" {
				t.Errorf("compile-time assertion for ClaudeClient still exists in interfaces.go - it should be removed along with the interface")
			}
		}
	}
}

// TestCompileTimeChecksExcludeClaudeClient verifies that the compile-time
// checks in interfaces.go do not include ClaudeClient after removal.
//
// Expected failure: The var block in interfaces.go still contains ClaudeClient check
func TestCompileTimeChecksExcludeClaudeClient(t *testing.T) {
	interfacesPath := filepath.Join("interfaces.go")

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, interfacesPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse interfaces.go: %v", err)
	}

	// Find compile-time checks - looking for the specific pattern in the file
	var foundChecksBlock bool
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		// Check if this is the compile-time checks block (has comment about satisfaction checks)
		if genDecl.Doc != nil {
			for _, comment := range genDecl.Doc.List {
				if strings.Contains(comment.Text, "Compile-time interface satisfaction") {
					foundChecksBlock = true
					// Count how many checks are in this block
					checksCount := len(genDecl.Specs)

					// After ClaudeClient removal, should be 5 checks (BeadClient, FailureAnalyzer, PromptRenderer, IterationLogger, WorktreeManager)
					// Currently there are 6 (including ClaudeClient)
					if checksCount >= 6 {
						t.Errorf("Compile-time checks block has %d checks - expected 5 after ClaudeClient removal (BeadClient, FailureAnalyzer, PromptRenderer, IterationLogger, WorktreeManager)", checksCount)
					}
				}
			}
		}
	}

	if !foundChecksBlock {
		t.Errorf("Could not find compile-time interface satisfaction checks block in interfaces.go")
	}
}

// TestInterfacesFileDoesNotImportClaude verifies that interfaces.go
// does not import the claude package after ClaudeClient is removed.
//
// Expected failure: interfaces.go still imports claude package
func TestInterfacesFileDoesNotImportClaude(t *testing.T) {
	interfacesPath := filepath.Join("interfaces.go")

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, interfacesPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse interfaces.go: %v", err)
	}

	// Check imports
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if strings.HasSuffix(importPath, "/internal/claude") {
			t.Errorf("interfaces.go still imports claude package - after removing ClaudeClient interface, the claude import should also be removed")
		}
	}
}

// TestMockClaudeClientRemovedFromTests verifies that mockClaudeClient
// is removed from interfaces_test.go since ClaudeClient interface no longer exists.
//
// Expected failure: mockClaudeClient struct still exists in interfaces_test.go
func TestMockClaudeClientRemovedFromTests(t *testing.T) {
	testPath := filepath.Join("interfaces_test.go")

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, testPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse interfaces_test.go: %v", err)
	}

	// Check that mockClaudeClient is not defined
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

			if typeSpec.Name.Name == "mockClaudeClient" {
				t.Errorf("mockClaudeClient still exists in interfaces_test.go - it should be removed because the ClaudeClient interface it mocks no longer exists")
			}
		}
	}
}

// TestInterfaceCountReducedAfterRemoval verifies that interfaces.go
// has fewer interface definitions after ClaudeClient is removed.
//
// Expected failure: interfaces.go still has 5 interfaces (including ClaudeClient)
func TestInterfaceCountReducedAfterRemoval(t *testing.T) {
	interfacesPath := filepath.Join("interfaces.go")

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, interfacesPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse interfaces.go: %v", err)
	}

	interfaceCount := 0
	interfaceNames := []string{}

	// Count interface definitions
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

			if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				interfaceCount++
				interfaceNames = append(interfaceNames, typeSpec.Name.Name)
			}
		}
	}

	// After removal, should have 5 interfaces: BeadClient, FailureAnalyzer, PromptRenderer, IterationLogger, WorktreeManager
	// Currently has 6 interfaces (including ClaudeClient)
	if interfaceCount != 5 {
		t.Errorf("interfaces.go has %d interfaces (%v), expected 5 after ClaudeClient removal (BeadClient, FailureAnalyzer, PromptRenderer, IterationLogger, WorktreeManager)", interfaceCount, interfaceNames)
	}

	// Verify ClaudeClient is not in the list
	for _, name := range interfaceNames {
		if name == "ClaudeClient" {
			t.Errorf("ClaudeClient is still present in interfaces.go - it should be removed")
		}
	}
}
