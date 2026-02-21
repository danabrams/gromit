package testutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// TaggedHarnessContext captures required paths for contract/e2e harness execution.
type TaggedHarnessContext struct {
	WorkingDir   string
	FakesDir     string
	FixturesDir  string
	CmdGromitDir string
}

// ResolveTaggedHarnessContext validates required invocation context for tagged harness suites.
// It expects to be called from test/contracts or test/e2e and returns canonical helper paths.
func ResolveTaggedHarnessContext(workingDir string) (TaggedHarnessContext, error) {
	if workingDir == "" {
		return TaggedHarnessContext{}, fmt.Errorf("working directory is empty")
	}

	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return TaggedHarnessContext{}, fmt.Errorf("resolve working directory %q: %w", workingDir, err)
	}

	leaf := filepath.Base(absWorkingDir)
	parent := filepath.Base(filepath.Dir(absWorkingDir))
	if parent != "test" || (leaf != "contracts" && leaf != "e2e") {
		return TaggedHarnessContext{}, fmt.Errorf("unexpected working directory %q (expected .../test/contracts or .../test/e2e)", absWorkingDir)
	}

	ctx := TaggedHarnessContext{
		WorkingDir:   absWorkingDir,
		FakesDir:     filepath.Join(filepath.Dir(absWorkingDir), "fakes"),
		FixturesDir:  filepath.Join(filepath.Dir(absWorkingDir), "fixtures"),
		CmdGromitDir: filepath.Join(absWorkingDir, "..", "..", "cmd", "gromit"),
	}

	if err := assertExistingDir(ctx.FakesDir, "fakes dir"); err != nil {
		return TaggedHarnessContext{}, err
	}
	if err := assertExistingDir(ctx.FixturesDir, "fixtures dir"); err != nil {
		return TaggedHarnessContext{}, err
	}
	if err := assertExistingDir(ctx.CmdGromitDir, "cmd/gromit dir"); err != nil {
		return TaggedHarnessContext{}, err
	}

	return ctx, nil
}

func assertExistingDir(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q not found: %w", label, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %q is not a directory", label, path)
	}
	return nil
}
