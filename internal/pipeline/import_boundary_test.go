package pipeline

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestPipelineImportBoundary guards the import boundary so pipeline code never
// depends on cmd/ packages. Regression would surface as a compile-time failure
// triggered by this guard instead of runtime leakage.
func TestPipelineImportBoundary(t *testing.T) {
	t.Parallel()
	assertPipelinePackagesDoNotImportCmd(t)
}

func assertPipelinePackagesDoNotImportCmd(t *testing.T) {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedImports}
	pkgs, err := packages.Load(cfg, "github.com/danabrams/gromit/internal/pipeline/...")
	if err != nil {
		t.Fatalf("load pipeline packages: %v", err)
	}
	var violations []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, pkgErr := range pkg.Errors {
				t.Fatalf("could not load %s: %v", pkg.PkgPath, pkgErr)
			}
		}
		if !strings.HasPrefix(pkg.PkgPath, "github.com/danabrams/gromit/internal/pipeline") {
			continue
		}
		for _, imp := range pkg.Imports {
			if imp == nil {
				continue
			}
			if strings.HasPrefix(imp.PkgPath, "github.com/danabrams/gromit/cmd") || strings.HasPrefix(imp.PkgPath, "cmd/") {
				violations = append(violations, fmt.Sprintf("%s -> %s", pkg.PkgPath, imp.PkgPath))
			}
		}
	}
	if len(violations) == 0 {
		return
	}
	sort.Strings(violations)
	t.Fatalf("pipeline packages import cmd packages:\n%s", strings.Join(violations, "\n"))
}
