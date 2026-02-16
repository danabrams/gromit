package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readRunnerTestFile(t *testing.T, rel string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestValidationAcceptanceFileSurfaceOnly(t *testing.T) {
	src := readRunnerTestFile(t, "validation_extraction_acceptance_test.go")

	forbidden := []string{
		"runValidationWithRecovery(",
		"runValidation(",
		"runDirectValidationCheck(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("validation_extraction_acceptance_test.go contains internal-behavior token %q", token)
		}
	}

	if !strings.Contains(src, "TestRunnerAcceptanceSurfaceOnly_ValidationFlow") {
		t.Fatal("validation_extraction_acceptance_test.go missing surface-only acceptance test")
	}
}

func TestBuildRouterPrimaryAcceptanceFileSurfaceOnly(t *testing.T) {
	src := readRunnerTestFile(t, "build_router_from_config_acceptance_test.go")

	forbidden := []string{
		"runner.router.Select(",
		"setupTwoProviderConfig(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("build_router_from_config_acceptance_test.go contains internal-behavior token %q", token)
		}
	}

	if !strings.Contains(src, "TestRunnerAcceptanceSurfaceOnly_RouterSelection") {
		t.Fatal("build_router_from_config_acceptance_test.go missing surface-only acceptance test")
	}
}

func TestBuildRouterAdditionalAcceptanceFileSurfaceOnly(t *testing.T) {
	src := readRunnerTestFile(t, "build_router_from_config_additional_acceptance_test.go")

	forbidden := []string{
		"runner.router.Select(",
		"setupSingleProviderConfig(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("build_router_from_config_additional_acceptance_test.go contains internal-behavior token %q", token)
		}
	}

	if !strings.Contains(src, "TestRunnerAcceptanceSurfaceOnly_RouterFallback") {
		t.Fatal("build_router_from_config_additional_acceptance_test.go missing surface-only acceptance test")
	}
}
