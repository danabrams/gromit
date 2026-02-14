package runner

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func parseImports(t *testing.T, path string) map[string]bool {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	imports := make(map[string]bool)
	for _, imp := range file.Imports {
		pathValue := imp.Path.Value
		if len(pathValue) < 2 {
			continue
		}
		imports[pathValue[1:len(pathValue)-1]] = true
	}

	return imports
}

func TestRunnerSplitPhase3_GatesExtracted(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)

	gatesPath := filepath.Join(runnerDir, "gates.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	if _, err := os.Stat(gatesPath); err != nil {
		t.Fatalf("expected gates.go to exist: %v", err)
	}

	gatesDecls := parseDecls(t, gatesPath)
	if gatesDecls.methods["Runner"] == nil || !gatesDecls.methods["Runner"]["runPrecheck"] {
		t.Fatal("gates.go missing method Runner.runPrecheck")
	}
	if gatesDecls.methods["Runner"] == nil || !gatesDecls.methods["Runner"]["checkScope"] {
		t.Fatal("gates.go missing method Runner.checkScope")
	}

	gatesImports := parseImports(t, gatesPath)
	required := []string{
		"context",
		"strings",
		"time",
		"github.com/danabrams/gromit/internal/bead",
		"github.com/danabrams/gromit/internal/prompt",
		"github.com/danabrams/gromit/internal/provider",
	}
	for _, importPath := range required {
		if !gatesImports[importPath] {
			t.Fatalf("gates.go missing import %q", importPath)
		}
	}

	runnerDecls := parseDecls(t, runnerPath)
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["runPrecheck"] {
		t.Fatal("runner.go still contains method Runner.runPrecheck")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["checkScope"] {
		t.Fatal("runner.go still contains method Runner.checkScope")
	}
}

func TestRunnerSplitPhase3_LoggingExtracted(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	runnerDir := filepath.Dir(thisFile)

	loggingPath := filepath.Join(runnerDir, "logging.go")
	runnerPath := filepath.Join(runnerDir, "runner.go")

	if _, err := os.Stat(loggingPath); err != nil {
		t.Fatalf("expected logging.go to exist: %v", err)
	}

	loggingDecls := parseDecls(t, loggingPath)
	if loggingDecls.methods["Runner"] == nil || !loggingDecls.methods["Runner"]["writeIterationLog"] {
		t.Fatal("logging.go missing method Runner.writeIterationLog")
	}
	if loggingDecls.methods["Runner"] == nil || !loggingDecls.methods["Runner"]["logResult"] {
		t.Fatal("logging.go missing method Runner.logResult")
	}
	if loggingDecls.methods["Runner"] == nil || !loggingDecls.methods["Runner"]["log"] {
		t.Fatal("logging.go missing method Runner.log")
	}

	loggingImports := parseImports(t, loggingPath)
	required := []string{
		"fmt",
		"strings",
		"time",
		"github.com/danabrams/gromit/internal/logger",
	}
	for _, importPath := range required {
		if !loggingImports[importPath] {
			t.Fatalf("logging.go missing import %q", importPath)
		}
	}

	runnerDecls := parseDecls(t, runnerPath)
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["writeIterationLog"] {
		t.Fatal("runner.go still contains method Runner.writeIterationLog")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["logResult"] {
		t.Fatal("runner.go still contains method Runner.logResult")
	}
	if runnerDecls.methods["Runner"] != nil && runnerDecls.methods["Runner"]["log"] {
		t.Fatal("runner.go still contains method Runner.log")
	}
}
