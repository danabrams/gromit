//go:build acceptance

package runner

import (
	"os"
	"strings"
	"testing"
)

func TestRunnerAcceptanceSurfaceOnly_RouterFallback(t *testing.T) {
	runner, err := NewRunner(routerAcceptanceConfig(t), os.Stdout)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if runner == nil {
		t.Fatal("expected non-nil runner")
	}
	if runner.analyzer == nil {
		t.Fatal("expected analyzer wiring for provider config")
	}
}

func TestRunnerAcceptanceReducedCountRouterAdditional(t *testing.T) {
	src := readRunnerTestFile(t, "build_router_from_config_additional_acceptance_test.go")
	if count := strings.Count(src, "\nfunc Test"); count > 3 {
		t.Fatalf("build_router_from_config_additional_acceptance_test.go contains %d tests; expected <= 3", count)
	}
}
