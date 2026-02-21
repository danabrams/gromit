//go:build acceptance

package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var golangciLintLookPath = exec.LookPath

func golangciLintCombinedOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func golangciLintAcceptanceEnv(t *testing.T) []string {
	t.Helper()

	cacheRoot := t.TempDir()
	return append(
		os.Environ(),
		"GOLANGCI_LINT_CACHE="+filepath.Join(cacheRoot, "golangci-lint-cache"),
		"GOCACHE="+filepath.Join(cacheRoot, "go-build-cache"),
	)
}

func resolveGolangCILintV2Path(t *testing.T) string {
	t.Helper()

	candidates := make([]string, 0, 3)
	if path, err := golangciLintLookPath("golangci-lint"); err == nil {
		candidates = append(candidates, path)
	}

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		candidates = append(candidates, filepath.Join(gopath, "bin", "golangci-lint"))
	} else if out, err := golangciLintCombinedOutput("go", "env", "GOPATH"); err == nil {
		gopath := strings.TrimSpace(string(out))
		if gopath != "" {
			candidates = append(candidates, filepath.Join(gopath, "bin", "golangci-lint"))
		}
	}

	seen := make(map[string]struct{})
	versionInfo := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}

		out, err := golangciLintCombinedOutput(candidate, "--version")
		version := strings.TrimSpace(string(out))
		if err != nil {
			versionInfo = append(versionInfo, candidate+": unable to read version")
			continue
		}

		versionInfo = append(versionInfo, candidate+": "+version)
		if strings.Contains(strings.ToLower(version), "version 2.") {
			return candidate
		}
	}

	t.Fatalf("golangci-lint v2 is required; searched %v; discovered versions: %v", candidates, versionInfo)
	return ""
}
