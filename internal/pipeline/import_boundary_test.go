package pipeline

import (
	"go/build"
	"path/filepath"
	"testing"
)

// TestImportBoundary_PipelineDoesNotImportCmd verifies that the pipeline package
// does not import the cmd/gromit package, enforcing the architectural boundary.
//
// This test prevents business logic from leaking from cmd/ into pipeline/,
// which would violate the CLI adapter pattern where cmd/ delegates to pipeline/.
func TestImportBoundary_PipelineDoesNotImportCmd(t *testing.T) {
	t.Parallel()

	// Get the pipeline package
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("failed to import pipeline package: %v", err)
	}

	// Check all imports for cmd/gromit references
	for _, imp := range pkg.Imports {
		if imp == "github.com/danabrams/gromit/cmd/gromit" {
			t.Fatalf("pipeline package must not import cmd/gromit (boundary violation)")
		}
		// Also check for direct cmd imports
		if filepath.HasPrefix(imp, "cmd/") {
			t.Fatalf("pipeline package must not import cmd/* packages (found %s)", imp)
		}
	}
}

// TestImportBoundary_PipelineDoesNotImportCmdPackageDir ensures pipeline never imports
// the cmd/gromit package specifically (as opposed to just having "cmd" in a path).
func TestImportBoundary_PipelineDoesNotImportCmdPackageDir(t *testing.T) {
	t.Parallel()

	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("failed to import pipeline package: %v", err)
	}

	forbiddenPatterns := []string{
		"cmd/gromit",
		"github.com/danabrams/gromit/cmd/gromit",
		"github.com/danabrams/gromit/cmd",
	}

	for _, imp := range pkg.Imports {
		for _, forbidden := range forbiddenPatterns {
			if imp == forbidden || filepath.HasPrefix(imp, forbidden) {
				t.Fatalf("pipeline package must not import cmd/ packages (found: %s)", imp)
			}
		}
	}
}
