package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/specgate"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/tracker/trackertest"
)

const (
	verifySpecSingleCriterionSpec = `---
id: my-spec
---

# My Spec

## Acceptance Criteria

- First criterion
`
	verifySpecTwoCriteriaSpec = `---
id: my-spec
---

# My Spec

## Acceptance Criteria

- First criterion
- Second criterion
`
)

func TestVerifySpecCmd_Registration(t *testing.T) {
	// Not parallel: reads rootCmd.Commands() which races with tests that mutate rootCmd.
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "verify-spec <spec>" {
			found = true
			break
		}
	}

	if !found {
		t.Error("verify-spec command not registered with rootCmd")
	}
}

func TestVerifySpecCmd_Flags(t *testing.T) {
	t.Parallel()
	flag := verifySpecCmd.Flags().Lookup("create-beads")
	if flag == nil {
		t.Error("verify-spec should have --create-beads flag")
		return
	}
	if flag.Value.Type() != "bool" {
		t.Errorf("--create-beads flag should be bool, got %s", flag.Value.Type())
	}
}

func TestVerifySpecCmd_ArgParsing(t *testing.T) {
	// Not parallel: runGromitCobra mutates rootCmd, os.Stdout, and os.Stderr.
	_, stderr, exitCode := runGromitCobra(t, "verify-spec")
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr, "accepts 1 arg(s), received 0") {
		t.Fatalf("stderr = %q, want argument count error", stderr)
	}
}

func TestExtractAcceptanceCriteria(t *testing.T) {
	t.Parallel()
	body := `# Title

## Acceptance Criteria

- First criterion
- Second criterion

## Decisions

- Not part of criteria
`

	criteria, block := extractAcceptanceCriteria(body)
	if len(criteria) != 2 {
		t.Fatalf("criteria count = %d, want 2", len(criteria))
	}
	if criteria[0] != "First criterion" {
		t.Errorf("criteria[0] = %q, want %q", criteria[0], "First criterion")
	}
	if criteria[1] != "Second criterion" {
		t.Errorf("criteria[1] = %q, want %q", criteria[1], "Second criterion")
	}
	if !strings.Contains(block, "- First criterion") {
		t.Errorf("block missing first criterion: %q", block)
	}
	if strings.Contains(block, "Not part of criteria") {
		t.Errorf("block should not include subsequent sections: %q", block)
	}
}

func TestVerifySpecCmd_OutputTableFormat(t *testing.T) {
	setupVerifySpecTest(t, "my-spec", verifySpecTwoCriteriaSpec)

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed: true,
			Results: []specgate.CriterionResult{
				{Criterion: "First criterion", Passed: true, Evidence: "covered by unit tests"},
				{Criterion: "Second criterion", Passed: false, Evidence: "missing assertion"},
			},
		}, nil
	}
	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
	})

	stdout, stderr, exitCode := runGromitCobra(t, "verify-spec", "my-spec")
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", exitCode, stderr)
	}
	if !strings.Contains(stdout, "CRITERION") || !strings.Contains(stdout, "STATUS") || !strings.Contains(stdout, "EVIDENCE") {
		t.Fatalf("stdout missing table header columns, got: %q", stdout)
	}
	if !strings.Contains(stdout, "First criterion") || !strings.Contains(stdout, "PASS") || !strings.Contains(stdout, "covered by unit tests") {
		t.Fatalf("stdout missing PASS row details, got: %q", stdout)
	}
	if !strings.Contains(stdout, "Second criterion") || !strings.Contains(stdout, "FAIL") || !strings.Contains(stdout, "missing assertion") {
		t.Fatalf("stdout missing FAIL row details, got: %q", stdout)
	}
}

func TestRunVerifySpec_PassReturnsNil(t *testing.T) {
	setupVerifySpecTest(t, "my-spec", verifySpecSingleCriterionSpec)

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		if specName != "my-spec" {
			t.Fatalf("specName = %q, want %q", specName, "my-spec")
		}
		if len(criteria) != 1 || criteria[0] != "First criterion" {
			t.Fatalf("criteria = %v, want [First criterion]", criteria)
		}
		return &specgate.GateVerdict{
			Passed:  true,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: true, Evidence: "ok"}},
		}, nil
	}
	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
	})

	verifySpecCreateBeads = false
	if err := runVerifySpec(verifySpecCmd, []string{"my-spec"}); err != nil {
		t.Fatalf("runVerifySpec returned error: %v", err)
	}
}

func TestVerifySpecCmd_ExitCodePassAndFail(t *testing.T) {
	setupVerifySpecTest(t, "my-spec", verifySpecSingleCriterionSpec)

	prevRunner := verifySpecGateRunner
	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
	})

	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed:  true,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: true, Evidence: "ok"}},
		}, nil
	}
	_, stderr, exitCode := runGromitCobra(t, "verify-spec", "my-spec")
	if exitCode != 0 {
		t.Fatalf("pass case exit code = %d, want 0 (stderr: %s)", exitCode, stderr)
	}

	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed:  false,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: false, Evidence: "missing"}},
		}, nil
	}
	_, stderr, exitCode = runGromitCobra(t, "verify-spec", "my-spec")
	if exitCode != 1 {
		t.Fatalf("fail case exit code = %d, want 1 (stderr: %s)", exitCode, stderr)
	}
	if !strings.Contains(stderr, "spec gate failed") {
		t.Fatalf("stderr = %q, want spec gate failed", stderr)
	}
}

func TestRunVerifySpec_CreateBeadsOnFailure(t *testing.T) {
	setupVerifySpecTest(t, "my-spec", verifySpecSingleCriterionSpec)

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed:  false,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: false, Evidence: "missing"}},
		}, nil
	}
	prevCreate := verifySpecFixBeadsFn
	called := false
	verifySpecFixBeadsFn = func(ctx context.Context, specName string, verdict *specgate.GateVerdict) ([]string, error) {
		called = true
		return []string{"gromit-123"}, nil
	}
	verifySpecCreateBeads = true

	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
		verifySpecFixBeadsFn = prevCreate
		verifySpecCreateBeads = false
	})

	if err := runVerifySpec(verifySpecCmd, []string{"my-spec"}); err == nil {
		t.Fatal("expected error on gate failure")
	}
	if !called {
		t.Fatal("expected fix beads creation on failure")
	}
}

func TestRunVerifySpec_DoesNotCreateBeadsWithoutFlag(t *testing.T) {
	setupVerifySpecTest(t, "my-spec", verifySpecSingleCriterionSpec)

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed:  false,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: false, Evidence: "missing"}},
		}, nil
	}
	prevCreate := verifySpecFixBeadsFn
	called := false
	verifySpecFixBeadsFn = func(ctx context.Context, specName string, verdict *specgate.GateVerdict) ([]string, error) {
		called = true
		return []string{"gromit-123"}, nil
	}
	verifySpecCreateBeads = false

	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
		verifySpecFixBeadsFn = prevCreate
		verifySpecCreateBeads = false
	})

	if err := runVerifySpec(verifySpecCmd, []string{"my-spec"}); err == nil {
		t.Fatal("expected error on gate failure")
	}
	if called {
		t.Fatal("did not expect fix beads creation when --create-beads is disabled")
	}
}

func TestVerifySpecCmd_CreateBeadsPrintsIDs(t *testing.T) {
	setupVerifySpecTest(t, "my-spec", verifySpecSingleCriterionSpec)

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed:  false,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: false, Evidence: "missing"}},
		}, nil
	}
	prevCreate := verifySpecFixBeadsFn
	verifySpecFixBeadsFn = func(ctx context.Context, specName string, verdict *specgate.GateVerdict) ([]string, error) {
		return []string{"gromit-a", "gromit-b"}, nil
	}

	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
		verifySpecFixBeadsFn = prevCreate
		verifySpecCreateBeads = false
	})

	stdout, _, exitCode := runGromitCobra(t, "verify-spec", "my-spec", "--create-beads")
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stdout, "Created fix beads: gromit-a, gromit-b") {
		t.Fatalf("stdout = %q, want created bead IDs", stdout)
	}
}

func TestBuildVerifySpecRouter_UsesProviderBuildRouterFromConfig(t *testing.T) {
	t.Parallel()
	verifySource, err := os.ReadFile("verify_spec.go")
	if err != nil {
		t.Fatalf("Reading verify_spec.go: %v", err)
	}

	sourceStr := string(verifySource)
	buildRouterFn, ok := extractFunction(sourceStr, "buildVerifySpecRouter")
	if !ok {
		t.Fatal("Cannot find buildVerifySpecRouter function")
	}

	if !strings.Contains(buildRouterFn, "provider.BuildRouterFromConfig(cfg)") {
		t.Error("buildVerifySpecRouter missing provider.BuildRouterFromConfig(cfg) call")
	}
}

func TestBuildVerifySpecRouter_UsesInjectedProviderBuilder(t *testing.T) {

	called := false
	prevBuilder := verifySpecBuildRouterFromConfig
	verifySpecBuildRouterFromConfig = func(cfg *config.Config) (*provider.Router, error) {
		called = true
		return nil, nil
	}
	t.Cleanup(func() {
		verifySpecBuildRouterFromConfig = prevBuilder
	})

	if _, err := buildVerifySpecRouter(&config.Config{}); err != nil {
		t.Fatalf("buildVerifySpecRouter returned error: %v", err)
	}
	if !called {
		t.Fatal("expected verifySpecBuildRouterFromConfig to be called")
	}
}

func TestVerifySpec_LocalProviderConfigHelpersRemoved(t *testing.T) {
	t.Parallel()
	verifySource, err := os.ReadFile("verify_spec.go")
	if err != nil {
		t.Fatalf("Reading verify_spec.go: %v", err)
	}

	sourceStr := string(verifySource)
	forbidden := []string{
		"func buildVerifySpecProviders(",
		"func defaultVerifySpecTierToModelMap(",
		"func defaultVerifySpecCodexTierToModelMap(",
		"func parseVerifySpecFallbackCooldown(",
		"buildVerifySpecProviders(cfg)",
		"defaultVerifySpecTierToModelMap()",
		"defaultVerifySpecCodexTierToModelMap()",
		"parseVerifySpecFallbackCooldown(cfg)",
	}
	for _, snippet := range forbidden {
		if strings.Contains(sourceStr, snippet) {
			t.Fatalf("verify_spec.go contains obsolete local config/provider helper usage: %q", snippet)
		}
	}
}

func TestAdapters_MarshalJSONListRemoved(t *testing.T) {
	t.Parallel()

	// Verify that marshalJSONList is not defined in adapters.go
	// This ensures we've consolidated to use tracker.EncodeMetadataJSONList
	source, err := os.ReadFile("adapters.go")
	if err != nil {
		t.Fatalf("failed to read adapters.go: %v", err)
	}

	sourceStr := string(source)
	if strings.Contains(sourceStr, "func marshalJSONList(") {
		t.Fatal("marshalJSONList function should be removed - use tracker.EncodeMetadataJSONList instead")
	}
}

func TestSpecGateBeadCreatorWithTrackerClient_EncodeLabelsWithTrackerFunction(t *testing.T) {
	t.Parallel()

	// Test that specGateBeadCreatorWithTrackerClient.Create encodes labels
	// using the same function as tracker.EncodeMetadataJSONList
	testCases := []struct {
		name   string
		labels []string
	}{
		{
			name:   "non-empty labels",
			labels: []string{"label1", "label2"},
		},
		{
			name:   "empty labels",
			labels: []string{},
		},
		{
			name:   "single label",
			labels: []string{"single"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a stub tracker client that captures the CreateRequest
			var capturedReq tracker.CreateRequest
			stubClient := &trackertest.StubTrackerClient{
				CreateFn: func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
					capturedReq = req
					return &tracker.Item{
						ID:    "test-id",
						Title: req.Title,
					}, nil
				},
			}

			creator := &specGateBeadCreatorWithTrackerClient{
				trackerClient: stubClient,
			}

			_, err := creator.Create(context.Background(), "Test Title", "Test Description", "P1", tc.labels)
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}

			// Verify labels encoding matches tracker.EncodeMetadataJSONList
			expectedLabels, labelsOk := tracker.EncodeMetadataJSONList(tc.labels)
			actualLabels, labelsPresent := capturedReq.Metadata["labels"]
			if labelsOk {
				if !labelsPresent || actualLabels != expectedLabels {
					t.Fatalf("labels mismatch: got %q, want %q", actualLabels, expectedLabels)
				}
			} else {
				if labelsPresent {
					t.Fatalf("labels should not be present when EncodeMetadataJSONList returns false")
				}
			}
		})
	}
}

func TestDefaultVerifySpecCmdRunner_CanceledContextReapsChildren(t *testing.T) {
	t.Parallel()

	pidFile := filepath.Join(t.TempDir(), "child.pid")
	command := fmt.Sprintf("sleep 30 & echo $! > %q; wait", pidFile)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, _, _, err := defaultVerifySpecCmdRunner(ctx, command, "")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}

	var childPID int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pidBytes, readErr := os.ReadFile(pidFile)
		if readErr == nil {
			parsedPID, parseErr := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
			if parseErr != nil {
				t.Fatalf("parsing child pid: %v", parseErr)
			}
			childPID = parsedPID
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if childPID <= 0 {
		t.Fatal("did not capture child pid before timeout")
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(childPID, 0); err != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child process still alive after canceled verify-spec command, pid=%d", childPID)
}

func setupVerifySpecTest(t *testing.T, specName string, specContent string) {
	t.Helper()

	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, specName+".md"), []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := "paths:\n  specs: " + specsDir + "\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	t.Chdir(tmpDir)
}
