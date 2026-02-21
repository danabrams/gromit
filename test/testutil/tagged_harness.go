package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	testDirName      = "test"
	contractsDirName = "contracts"
	e2eDirName       = "e2e"
	fakesDirName     = "fakes"
	fixturesDirName  = "fixtures"
	cmdDirName       = "cmd"
	gromitDirName    = "gromit"
)

// TaggedHarnessContext captures required paths for contract/e2e harness execution.
type TaggedHarnessContext struct {
	WorkingDir   string
	FakesDir     string
	FixturesDir  string
	CmdGromitDir string
}

// ResolveTaggedHarnessContext validates required invocation context for tagged harness suites.
// It supports invocation from either repository root or test/contracts|test/e2e package directories.
func ResolveTaggedHarnessContext(workingDir string) (TaggedHarnessContext, error) {
	if workingDir == "" {
		return TaggedHarnessContext{}, fmt.Errorf("working directory is empty")
	}

	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return TaggedHarnessContext{}, fmt.Errorf("resolve working directory %q: %w", workingDir, err)
	}

	repoRoot, err := resolveTaggedHarnessRepoRoot(absWorkingDir)
	if err != nil {
		return TaggedHarnessContext{}, err
	}
	testDir := filepath.Join(repoRoot, testDirName)

	ctx := TaggedHarnessContext{
		WorkingDir:   absWorkingDir,
		FakesDir:     filepath.Join(testDir, fakesDirName),
		FixturesDir:  filepath.Join(testDir, fixturesDirName),
		CmdGromitDir: filepath.Join(repoRoot, cmdDirName, gromitDirName),
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

func resolveTaggedHarnessRepoRoot(absWorkingDir string) (string, error) {
	root := filepath.Clean(absWorkingDir)
	var attempted []string

	for {
		attempted = append(attempted, root)
		if hasTaggedHarnessLayout(root) {
			return root, nil
		}

		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}

	return "", fmt.Errorf(
		"unable to resolve tagged harness repo root from %q (attempted: %s)",
		absWorkingDir,
		strings.Join(attempted, ", "),
	)
}

func hasTaggedHarnessLayout(root string) bool {
	testDir := filepath.Join(root, testDirName)
	if !isExistingDir(filepath.Join(testDir, contractsDirName)) {
		return false
	}
	if !isExistingDir(filepath.Join(testDir, e2eDirName)) {
		return false
	}
	if !isExistingDir(filepath.Join(testDir, fakesDirName)) {
		return false
	}
	if !isExistingDir(filepath.Join(testDir, fixturesDirName)) {
		return false
	}
	if !isExistingDir(filepath.Join(root, cmdDirName, gromitDirName)) {
		return false
	}
	return true
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

func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
