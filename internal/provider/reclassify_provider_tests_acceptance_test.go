//go:build acceptance

package provider

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestProviderAcceptanceSurfaceOnlyStreamingE2E verifies the provider acceptance
// file contains only true StreamRun end-to-end behavior and excludes internal
// event parsing/wiring tests.
// Expected failure: AssertProviderAcceptanceReclassificationImplemented helper does not exist yet.
func TestProviderAcceptanceSurfaceOnlyStreamingE2E(t *testing.T) {
	AssertProviderAcceptanceReclassificationImplemented(t)

	acceptancePath := filepath.Join("internal", "provider", "codex_streaming_acceptance_test.go")
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
// Expected failure: AssertProviderAcceptanceReclassificationImplemented helper does not exist yet.
func TestProviderReclassifiedBehaviorRunsInUnitSuite(t *testing.T) {
	AssertProviderAcceptanceReclassificationImplemented(t)

	cmd := exec.Command("go", "test", "-list", "TestProcessCodexStream", "./internal/provider")
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
// Expected failure: codexProviderAcceptanceMaxTests constant does not exist yet.
func TestProviderAcceptanceSuiteHasReducedFootprint(t *testing.T) {
	maxAllowed := codexProviderAcceptanceMaxTests

	cmd := exec.Command("go", "test", "-tags", "acceptance", "-list", "^Test", "./internal/provider")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go test -tags acceptance -list failed: %v\noutput:\n%s", err, out.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var tests []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Test") {
			tests = append(tests, trimmed)
		}
	}

	if len(tests) > maxAllowed {
		t.Errorf("acceptance test footprint too large: got %d tests, max %d", len(tests), maxAllowed)
	}
}
