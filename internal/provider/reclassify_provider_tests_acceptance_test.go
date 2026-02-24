//go:build acceptance

package provider

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestProviderAcceptanceSurfaceOnlyStreamingE2E verifies the provider acceptance
// file contains only true StreamRun end-to-end behavior and excludes internal
// event parsing/wiring tests.
func TestProviderAcceptanceSurfaceOnlyStreamingE2E(t *testing.T) {
	repoRoot := AssertProviderAcceptanceReclassificationImplemented(t)

	acceptancePath := filepath.Join(repoRoot, "internal", "provider", "codex_streaming_acceptance_test.go")
	src, err := os.ReadFile(acceptancePath)
	if err != nil {
		t.Fatalf("read %s: %v", acceptancePath, err)
	}
	content := string(src)

	mustNotContain := []string{
		"processCodexStream(",
		"TestProcessCodexStream",
		"TestCodexEventStructsExist",
		"TestCodexUsageStructFields",
		"TestCodexItemStructSupportsMultipleTypes",
	}
	for _, disallowed := range mustNotContain {
		if strings.Contains(content, disallowed) {
			t.Errorf("%s should not contain internal behavior test marker %q", acceptancePath, disallowed)
		}
	}

	mustContain := []string{
		"NewCodexProvider(",
		"StreamRun(",
	}
	for _, required := range mustContain {
		if !strings.Contains(content, required) {
			t.Errorf("%s should keep streaming E2E behavior and include %q", acceptancePath, required)
		}
	}
}

// TestProviderReclassifiedBehaviorRunsInUnitSuite verifies internal parsing and
// wiring behavior tests are visible under normal go test (without acceptance tag).
func TestProviderReclassifiedBehaviorRunsInUnitSuite(t *testing.T) {
	repoRoot := AssertProviderAcceptanceReclassificationImplemented(t)

	cmd := exec.Command("go", "test", "-list", "TestProcessCodexStream", "./internal/provider")
	cmd.Dir = repoRoot
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test -list on unit suite failed: %v\noutput:\n%s", err, out.String())
	}

	listed := out.String()
	expectedUnitTests := []string{
		"TestProcessCodexStreamMapsThreadStartedToSystem",
		"TestProcessCodexStreamMapsAgentMessageToAssistant",
		"TestProcessCodexStreamToolCallHandlers",
	}
	for _, testName := range expectedUnitTests {
		if !strings.Contains(listed, testName) {
			t.Errorf("normal unit suite should list %s after reclassification", testName)
		}
	}
}

// TestProviderAcceptanceSuiteHasReducedFootprint verifies acceptance-tag runs in
// internal/provider are reduced to a slim E2E surface.
func TestProviderAcceptanceSuiteHasReducedFootprint(t *testing.T) {
	repoRoot := AssertProviderAcceptanceReclassificationImplemented(t)

	maxAllowed := codexProviderAcceptanceMaxTests

	baseline, err := listProviderTests(t, repoRoot, false)
	if err != nil {
		t.Fatalf("list baseline unit tests: %v", err)
	}

	withAcceptance, err := listProviderTests(t, repoRoot, true)
	if err != nil {
		t.Fatalf("list acceptance-tag test set: %v", err)
	}

	if len(withAcceptance) < len(baseline) {
		t.Fatalf("acceptance-tag run listed fewer tests than baseline: baseline=%d acceptance=%d", len(baseline), len(withAcceptance))
	}

	additional := len(withAcceptance) - len(baseline)
	if additional > maxAllowed {
		t.Errorf("acceptance-specific test footprint too large: got %d additional tests, max %d", additional, maxAllowed)
	}
}

func TestProviderAcceptanceSurfaceIncludesPhase3CacheFallbackIntegrationCase(t *testing.T) {
	repoRoot := AssertProviderAcceptanceReclassificationImplemented(t)

	tests, err := listProviderTests(t, repoRoot, false)
	if err != nil {
		t.Fatalf("list provider tests: %v", err)
	}

	found := false
	for _, name := range tests {
		if name == "TestResolveCacheAdapter_Integration_NilCapableAdapterFallsBackToNoop" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing provider phase-3 cache fallback integration test %q", "TestResolveCacheAdapter_Integration_NilCapableAdapterFallsBackToNoop")
	}
}

func listProviderTests(t *testing.T, repoRoot string, withAcceptance bool) ([]string, error) {
	t.Helper()

	args := []string{"test", "-list", "^Test"}
	if withAcceptance {
		args = append(args, "-tags", "acceptance")
	}
	args = append(args, "./internal/provider")

	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go %s failed: %w\noutput:\n%s", strings.Join(args, " "), err, out.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var tests []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Test") {
			tests = append(tests, trimmed)
		}
	}
	return tests, nil
}
