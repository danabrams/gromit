
package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func reclassifyRunnerDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(thisFile)
}

func reclassifyReadFile(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Join(reclassifyRunnerDir(t), rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func reclassifyCountTests(src string) int {
	return strings.Count(src, "\nfunc Test")
}

// TestReclassifyRunnerAcceptanceFilesSurfaceOnly verifies that the target
// acceptance files keep only command/API-surface coverage and remove
// internal behavior checks.
//
// Expected failure: acceptance files still assert internal facade behavior
// (for example runValidationWithRecovery and runner.router.Select) instead of
// only command/API-surface behavior.
func TestReclassifyRunnerAcceptanceFilesSurfaceOnly(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		forbidden []string
		required  []string
	}{
		{
			name: "validation extraction acceptance file",
			file: "validation_extraction_acceptance_test.go",
			forbidden: []string{
				"runValidationWithRecovery(",
				"runValidation(",
				"runDirectValidationCheck(",
			},
			required: []string{
				"TestRunnerAcceptanceSurfaceOnly_ValidationFlow",
			},
		},
		{
			name: "router acceptance file",
			file: "build_router_from_config_acceptance_test.go",
			forbidden: []string{
				"runner.router.Select(",
				"setupTwoProviderConfig(",
			},
			required: []string{
				"TestRunnerAcceptanceSurfaceOnly_RouterSelection",
			},
		},
		{
			name: "router additional acceptance file",
			file: "build_router_from_config_additional_acceptance_test.go",
			forbidden: []string{
				"runner.router.Select(",
				"setupSingleProviderConfig(",
			},
			required: []string{
				"TestRunnerAcceptanceSurfaceOnly_RouterFallback",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := reclassifyReadFile(t, tt.file)
			for _, token := range tt.forbidden {
				if strings.Contains(src, token) {
					t.Fatalf("%s contains internal-behavior token %q", tt.file, token)
				}
			}
			for _, name := range tt.required {
				if !strings.Contains(src, name) {
					t.Fatalf("%s is missing future surface-only coverage %q", tt.file, name)
				}
			}
		})
	}
}

// TestReclassifyRunnerUnitSuiteListsMovedBehavior verifies that moved
// behavior is visible in the default unit suite (without acceptance tag).
//
// Expected failure: reclassified unit tests are not yet present under the
// new names, so `go test ./internal/runner -list` does not include them.
func TestReclassifyRunnerUnitSuiteListsMovedBehavior(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join(reclassifyRunnerDir(t), "..", ".."))

	cmd := exec.Command("go", "test", "./internal/runner", "-list", "Test")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -list failed: %v\n%s", err, string(out))
	}

	listed := string(out)
	required := []string{
		"TestValidationExtractionReclassified_RunValidationDelegation",
		"TestBuildRouterReclassified_PhasePreferenceRouting",
		"TestBuildRouterReclassified_CooldownParsing",
	}
	for _, name := range required {
		if !strings.Contains(listed, name) {
			t.Fatalf("unit test listing missing %s", name)
		}
	}
}

// TestReclassifyRunnerAcceptanceSuiteReducedTargetCount verifies that the
// three target acceptance files are reduced to a small E2E-only surface.
//
// Expected failure: these files still contain many internal-behavior tests
// and exceed the reduced count budget.
func TestReclassifyRunnerAcceptanceSuiteReducedTargetCount(t *testing.T) {
	files := []string{
		"validation_extraction_acceptance_test.go",
		"build_router_from_config_acceptance_test.go",
		"build_router_from_config_additional_acceptance_test.go",
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			src := reclassifyReadFile(t, file)
			count := reclassifyCountTests(src)
			if count > 2 {
				t.Fatalf("%s contains %d tests; expected <= 2 after reclassification", file, count)
			}
		})
	}
}
