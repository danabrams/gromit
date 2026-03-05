//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/loop"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

func TestAdapterSwappability(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, swap := range adapterSwaps() {
		swap := swap
		t.Run(swap.name, func(t *testing.T) {
			loopInstance, err := loop.NewSpecLoop(swap.setup(newAdapterSet()), &config.Config{}, newDependencyGate())
			if err != nil {
				t.Fatalf("setup loop: %v", err)
			}
			if err := loopInstance.Run(ctx, "spec-swap"); err != nil {
				t.Fatalf("run loop: %v", err)
			}
		})
	}

	baseLoop, err := loop.NewSpecLoop(newAdapterSet(), &config.Config{}, newDependencyGate())
	if err != nil {
		t.Fatalf("base state: %v", err)
	}
	if err := baseLoop.Run(ctx, "spec-base"); err != nil {
		t.Fatalf("base run: %v", err)
	}

	assertStagePackagesAvoidAdapterImports(t)
	assertLoopImportsConfigPackage(t)
}

type adapterSwap struct {
	name  string
	setup func(loop.AdapterSet) loop.AdapterSet
}

func adapterSwaps() []adapterSwap {
	return []adapterSwap{
		{
			name: "git",
			setup: func(set loop.AdapterSet) loop.AdapterSet {
				set.Git = &fakeGitAdapter{}
				return set
			},
		},
		{
			name: "llm",
			setup: func(set loop.AdapterSet) loop.AdapterSet {
				set.LLM = &fakeLLMAdapter{}
				return set
			},
		},
		{
			name: "task-tracker",
			setup: func(set loop.AdapterSet) loop.AdapterSet {
				set.TaskTracker = &fakeTaskTrackerAdapter{}
				return set
			},
		},
		{
			name: "presenter",
			setup: func(set loop.AdapterSet) loop.AdapterSet {
				set.Presenter = &fakePresenterAdapter{}
				return set
			},
		},
	}
}

func newAdapterSet() loop.AdapterSet {
	return loop.AdapterSet{
		Git:         &fakeGitAdapter{},
		LLM:         &fakeLLMAdapter{},
		TaskTracker: &fakeTaskTrackerAdapter{},
		Presenter:   &fakePresenterAdapter{},
	}
}

func assertStagePackagesAvoidAdapterImports(t *testing.T) {
	t.Helper()
	stagePath := filepath.Join("stages")
	const adapterPrefix = "github.com/danabrams/gromit/internal/v2/adapter"

	info, err := os.Stat(stagePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stage package root %s missing", stagePath)
		}
		t.Fatalf("stat stage packages: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("stage root %s is not a directory", stagePath)
	}

	fset := token.NewFileSet()
	err = filepath.WalkDir(stagePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imp := range file.Imports {
			pathValue := strings.Trim(imp.Path.Value, "\"")
			if strings.HasPrefix(pathValue, adapterPrefix) {
				t.Fatalf("stage file %s imports adapter package %s", path, pathValue)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking stage packages: %v", err)
	}
}

func assertLoopImportsConfigPackage(t *testing.T) {
	t.Helper()
	loopPath := filepath.Join("loop")
	const configImport = "github.com/danabrams/gromit/internal/config"

	info, err := os.Stat(loopPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("loop package %s missing", loopPath)
		}
		t.Fatalf("stat loop package: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("loop path %s is not a directory", loopPath)
	}

	fset := token.NewFileSet()
	found := false
	err = filepath.WalkDir(loopPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imp := range file.Imports {
			if strings.Trim(imp.Path.Value, "\"") == configImport {
				found = true
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking loop package: %v", err)
	}
	if !found {
		t.Fatalf("loop package does not import %s", configImport)
	}
}

type fakeGitAdapter struct{}

func (fakeGitAdapter) Checkout(_ context.Context, specID string) (string, error) {
	return "/tmp/" + specID, nil
}

type fakeLLMAdapter struct{}

func (fakeLLMAdapter) GeneratePlan(_ context.Context, specID string) (string, error) {
	return specID + "-plan", nil
}

type fakeTaskTrackerAdapter struct{}

func (fakeTaskTrackerAdapter) RecordPlan(_ context.Context, specID, plan string) error {
	_ = specID
	_ = plan
	return nil
}

type fakePresenterAdapter struct{}

func (fakePresenterAdapter) PresentSummary(_ context.Context, specID string, summary presentation.PresentationSummary) error {
	_ = specID
	_ = summary
	return nil
}

type noopDependencyGate struct{}

func (noopDependencyGate) EnsureSpecReady(_ context.Context, _ string) error {
	return nil
}

func newDependencyGate() loop.DependencyGate {
	return noopDependencyGate{}
}
