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
		FakesDir:     filepath.Join(testDir, "fakes"),
		FixturesDir:  filepath.Join(testDir, "fixtures"),
		CmdGromitDir: filepath.Join(repoRoot, "cmd", "gromit"),
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
	if !isExistingDir(filepath.Join(testDir, "fakes")) {
		return false
	}
	if !isExistingDir(filepath.Join(testDir, "fixtures")) {
		return false
	}
	if !isExistingDir(filepath.Join(root, "cmd", "gromit")) {
		return false
	}
	return true
}

func isTaggedHarnessLeaf(leaf string) bool {
	return leaf == contractsDirName || leaf == e2eDirName
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
