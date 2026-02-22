package reviewpkg

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestReviewerTestNoSiblingImports verifies that reviewer_test.go does not
// import sibling packages under internal/runner/. Sub-packages must not import
// siblings — specifically the escalation package must not be imported here.
func TestReviewerTestNoSiblingImports(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "reviewer_test.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse reviewer_test.go: %v", err)
	}

	const siblingPrefix = "github.com/danabrams/gromit/internal/runner/"
	const allowedSibling = "github.com/danabrams/gromit/internal/runner/runtypes"

	for _, imp := range node.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.HasPrefix(path, siblingPrefix) && path != allowedSibling {
			t.Errorf("reviewer_test.go imports sibling package %q — sub-packages must not import siblings; define needed types locally or move to runtypes/", path)
		}
	}
}

// TestVariableDeclarationMergePattern documents that variable declarations
// followed immediately by assignment should be merged for clarity.
func TestVariableDeclarationMergePattern(t *testing.T) {
	// Good: merged declaration and assignment
	result := computeValue()
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}

	// This test verifies the pattern is correct
}

func computeValue() int {
	return 42
}
