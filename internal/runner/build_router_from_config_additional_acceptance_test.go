//go:build acceptance

package runner

import (
	"os"
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
