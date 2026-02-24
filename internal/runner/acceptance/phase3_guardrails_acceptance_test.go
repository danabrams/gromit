//go:build acceptance

package acceptance_test

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestPhase3Coverage_HasCacheDeterminismIntegrationCase(t *testing.T) {
	tests, err := listPackageTests(repoRoot(t), "./internal/runner/execution")
	if err != nil {
		t.Fatalf("list execution tests: %v", err)
	}

	const expected = "TestInvokerExecute_Integration_CacheKeyStableAcrossBuildContextIterations"
	if !tests[expected] {
		t.Fatalf("missing phase-3 cache determinism integration test %q", expected)
	}
}

func TestPhase3Coverage_HasCacheFailureFallbackIntegrationCase(t *testing.T) {
	tests, err := listPackageTests(repoRoot(t), "./internal/runner/execution")
	if err != nil {
		t.Fatalf("list execution tests: %v", err)
	}

	const expected = "TestInvokerExecute_Integration_CacheLookupFailureFallsBackWithoutAbort"
	if !tests[expected] {
		t.Fatalf("missing phase-3 cache fallback integration test %q", expected)
	}
}

func listPackageTests(root, pkg string) (map[string]bool, error) {
	cmd := exec.Command("go", "test", "-list", "^Test", pkg)
	cmd.Dir = root

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go test -list failed: %w\noutput:\n%s", err, out.String())
	}

	tests := map[string]bool{}
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Test") {
			tests[line] = true
		}
	}
	return tests, nil
}
