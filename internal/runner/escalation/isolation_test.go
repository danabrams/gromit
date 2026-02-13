package escalation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestEscalationHandlerHasLogFn verifies that the Handler struct accepts a
// LogFn callback for structured logging, matching the runtypes.LogFn type.
// After delegation, the Runner should pass its log function to the Handler
// so escalation events (retry attempts, tier changes, decomposition) are
// logged through the Runner's output writer.
//
// Expected failure: Handler struct does not currently have a logFn field.
// It was extracted with the minimal interface set and logging was left in
// the Runner's local methods. Once local methods are removed and the Runner
// delegates to escalation.Handler, the Handler needs a LogFn to produce
// the same log output the Runner's local methods produced.
func TestEscalationHandlerHasLogFn(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "handler.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse handler.go: %v", err)
	}

	// Look for the Handler struct and check for a logFn or log field
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "Handler" {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structType.Fields.List {
				for _, name := range field.Names {
					if name.Name == "logFn" || name.Name == "log" {
						return // found the log function field
					}
				}
			}
			t.Errorf("Handler struct in handler.go does not have a logFn field — " +
				"the Handler needs a logging callback so escalation events (retry attempts, " +
				"tier changes, decomposition) are logged through the Runner's output writer")
			return
		}
	}
	t.Fatal("Handler struct not found in handler.go")
}

// TestNewHandlerAcceptsLogFn verifies that NewHandler accepts a LogFn
// parameter for structured logging during escalation.
//
// Expected failure: NewHandler currently takes 5 params: (cfg, analyzer,
// beadClient, decomposeFn, createSubFn). After the Runner delegates fully,
// NewHandler should also accept a LogFn so the Handler can log retry/escalation
// events that the Runner's local methods currently log.
func TestNewHandlerAcceptsLogFn(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "handler.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse handler.go: %v", err)
	}

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name.Name != "NewHandler" {
			continue
		}
		// Count parameters
		params := funcDecl.Type.Params.List
		totalParams := 0
		for _, p := range params {
			if len(p.Names) == 0 {
				totalParams++
			} else {
				totalParams += len(p.Names)
			}
		}
		// Currently NewHandler has 5 params: cfg, analyzer, beadClient, decomposeFn, createSubFn
		// After adding logFn, it should have 6+ params
		if totalParams <= 5 {
			t.Errorf("NewHandler has %d parameters, want > 5 — needs a LogFn parameter "+
				"so escalation events are logged through the Runner's output writer", totalParams)
		}
		return
	}
	t.Fatal("NewHandler not found in handler.go")
}
