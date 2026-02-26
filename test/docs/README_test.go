package docs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestREADMEDocumentsLanesAndFixtures(t *testing.T) {
	readmePath := filepath.Join("..", "..", "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}

	checks := []string{
		"Default lane (fast)",
		"Smoke lane (real CLI)",
		"Fixture refresh workflow",
	}

	body := string(content)
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Fatalf("README.md missing %q", want)
		}
	}
}

func TestSmokeLaneTestsAreProperlyGated(t *testing.T) {
	// RED test: Verify that all smoke tests use //go:build smokecli
	// and that they check for CLAUDE_SMOKE and CODEX_SMOKE env vars
	projectRoot := filepath.Join("..", "..")

	smokeTestPath := filepath.Join(projectRoot, "internal", "provider", "claude_codex_smoke_test.go")
	content, err := os.ReadFile(smokeTestPath)
	if err != nil {
		t.Fatalf("failed to read smoke test file: %v", err)
	}

	body := string(content)

	// Verify build tag
	if !strings.Contains(body, "//go:build smokecli") {
		t.Error("smoke test file missing //go:build smokecli tag")
	}

	// Verify environment variable checks
	if !strings.Contains(body, "CLAUDE_SMOKE") {
		t.Error("smoke test file should check CLAUDE_SMOKE env var")
	}

	if !strings.Contains(body, "CODEX_SMOKE") {
		t.Error("smoke test file should check CODEX_SMOKE env var")
	}
}

func TestFastLaneDefaultTestStructure(t *testing.T) {
	// RED test: Verify that default lane (fast) test files don't require
	// smoke environment variables or real CLI binaries
	projectRoot := filepath.Join("..", "..")

	// Key test files that should run in default lane
	fastLaneTests := []string{
		"cmd/gromit/benchmark_test.go",
		"cmd/gromit/end_to_end_verification_test.go",
		"internal/provider/claude_test.go",
		"internal/provider/codex_test.go",
	}

	for _, testFile := range fastLaneTests {
		fullPath := filepath.Join(projectRoot, testFile)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			// Some files may not exist, that's OK
			continue
		}

		body := string(content)

		// Default lane tests should NOT require these env vars
		if strings.Contains(body, "os.Getenv(\"CLAUDE_SMOKE\")") && !strings.Contains(body, "//go:build smokecli") {
			t.Errorf("%s checks CLAUDE_SMOKE without //go:build smokecli tag", testFile)
		}

		if strings.Contains(body, "os.Getenv(\"CODEX_SMOKE\")") && !strings.Contains(body, "//go:build smokecli") {
			t.Errorf("%s checks CODEX_SMOKE without //go:build smokecli tag", testFile)
		}
	}
}

func TestLaneAwareBudgetsAreConfigured(t *testing.T) {
	// RED test: Verify that test_package_budgets.txt has lane-aware
	// budget entries (e.g., "package:lane budget_seconds")
	projectRoot := filepath.Join("..", "..")
	budgetPath := filepath.Join(projectRoot, "scripts", "test_package_budgets.txt")

	file, err := os.Open(budgetPath)
	if err != nil {
		t.Fatalf("failed to open budget file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	foundLaneBudgets := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Check if line has lane notation (package:lane budget)
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			pkgOrLane := parts[0]
			if strings.Contains(pkgOrLane, ":") {
				foundLaneBudgets = true

				// Verify budget is a valid number
				budget := parts[1]
				if _, err := strconv.Atoi(budget); err != nil {
					t.Errorf("invalid budget value %q in line %q", budget, line)
				}

				// Verify lane part is "default" or "smoke"
				laneParts := strings.Split(pkgOrLane, ":")
				if len(laneParts) == 2 {
					lane := laneParts[1]
					if lane != "default" && lane != "smoke" {
						t.Errorf("unknown lane %q in budget line %q", lane, line)
					}
				}
			}
		}
	}

	if !foundLaneBudgets {
		t.Error("test_package_budgets.txt should have lane-aware budget entries (package:lane format)")
	}

	if err := scanner.Err(); err != nil {
		t.Errorf("error reading budget file: %v", err)
	}
}

func TestFixtureRefreshWorkflowDocumented(t *testing.T) {
	// GREEN test: Verify that README documents the fixture refresh workflow
	// with all required steps
	projectRoot := filepath.Join("..", "..")
	readmePath := filepath.Join(projectRoot, "README.md")

	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}

	body := string(content)

	// Verify fixture refresh steps are documented
	requiredSteps := []string{
		"Capture the output",
		"Annotate the fixture",
		"Sanitize and stabilize",
		"Validate in the default lane",
		"Review with intent",
	}

	for _, step := range requiredSteps {
		if !strings.Contains(body, step) {
			t.Errorf("README.md missing fixture refresh step: %q", step)
		}
	}

	// Verify environment gate documentation
	if !strings.Contains(body, "CLAUDE_SMOKE") {
		t.Error("README.md should document CLAUDE_SMOKE environment gate")
	}

	if !strings.Contains(body, "CODEX_SMOKE") {
		t.Error("README.md should document CODEX_SMOKE environment gate")
	}
}

func TestLaneTimingScriptImplementsLaneDetection(t *testing.T) {
	// GREEN test: Verify that test_timing.sh script implements lane detection
	projectRoot := filepath.Join("..", "..")
	scriptPath := filepath.Join(projectRoot, "scripts", "test_timing.sh")

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read test_timing.sh: %v", err)
	}

	body := string(content)

	// Verify lane detection logic
	laneDetectionChecks := []string{
		"[Ss]moke",
		"smoke_",
		"default",
		"get_budget",
	}

	for _, check := range laneDetectionChecks {
		if !strings.Contains(body, check) {
			t.Errorf("test_timing.sh missing lane detection element: %q", check)
		}
	}

	// Verify per-lane budget handling
	if !strings.Contains(body, "pkg_lane_elapsed") {
		t.Error("test_timing.sh should track per-lane elapsed times")
	}

	// Verify lane-specific budget lookup with $pkg:$lane syntax
	if !strings.Contains(body, "$pkg:$lane") {
		t.Error("test_timing.sh should support lane-specific budgets using $pkg:$lane syntax")
	}
}
