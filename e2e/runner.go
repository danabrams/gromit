//go:build e2e

package e2e

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"gopkg.in/yaml.v3"
)

// ScenarioResult holds the outcome of running a scenario binary invocation.
type ScenarioResult struct {
	runID      string
	storeDir   string
	fixtureDir string
	exitCode   int
	stdout     string
}

var runIDRegex = regexp.MustCompile(`Run ID:\s+(run-[0-9a-f]+)`)

// LoadContracts reads all YAML contract files from the given directory.
func LoadContracts(t *testing.T, contractsDir string) []Contract {
	t.Helper()
	entries, err := os.ReadDir(contractsDir)
	if err != nil {
		t.Fatalf("read contracts dir %s: %v", contractsDir, err)
	}
	var contracts []Contract
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(contractsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read contract %s: %v", path, err)
		}
		var c Contract
		if err := yaml.Unmarshal(data, &c); err != nil {
			t.Fatalf("parse contract %s: %v", path, err)
		}
		contracts = append(contracts, c)
	}
	return contracts
}

// RequireE2E skips the test unless GROMIT_E2E=1 is set.
func RequireE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("GROMIT_E2E") != "1" {
		t.Skip("set GROMIT_E2E=1 to run e2e tests")
	}
}

// BuildBinary builds gromit-next from source and returns the path to the binary.
func BuildBinary(t *testing.T) string {
	t.Helper()
	out := t.TempDir()
	binary := filepath.Join(out, "gromit-next")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/gromit-next/")
	cmd.Dir = "/Users/dabrams/gromit"
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}
	return binary
}

// Slug converts a name into a test-friendly identifier.
func Slug(name string) string {
	r := strings.NewReplacer(" ", "_", "/", "_", "\u2014", "", "-", "_", "(", "", ")", "", ":", "", ",", "")
	s := r.Replace(name)
	// Collapse multiple underscores
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return strings.Trim(s, "_")
}

// findContractByScenario returns the contract with the given scenario number.
func findContractByScenario(t *testing.T, contractsDir string, scenario int) Contract {
	t.Helper()
	contracts := LoadContracts(t, contractsDir)
	for _, c := range contracts {
		if c.Scenario == scenario {
			return c
		}
	}
	t.Fatalf("no contract found for scenario %d", scenario)
	return Contract{}
}

// ResetFixture restores the fixture directory to a known state before running a scenario.
func ResetFixture(t *testing.T, c Contract, fixtureDir string) {
	t.Helper()

	// Restore files from git at specific commits.
	for _, gf := range c.FixtureReset.GitFiles {
		for _, file := range gf.Files {
			cmd := exec.Command("git", "checkout", gf.Commit, "--", file)
			cmd.Dir = fixtureDir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git checkout %s %s: %v\n%s", gf.Commit, file, err, out)
			}
		}
	}

	// Remove files.
	for _, f := range c.FixtureReset.RemoveFiles {
		path := filepath.Join(fixtureDir, f)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove fixture file %s: %v", path, err)
		}
	}

	// Copy files in.
	for _, fc := range c.FixtureReset.AddFiles {
		src := fc.Src
		if !filepath.IsAbs(src) {
			// src paths are relative to the gromit repo root (e.g. e2e/testdata/...)
			src = filepath.Join("/Users/dabrams/gromit", src)
		}
		dst := filepath.Join(fixtureDir, fc.Dst)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("create dir for %s: %v", dst, err)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read add file src %s: %v", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("write add file dst %s: %v", dst, err)
		}
	}
}

// writePolicyFile writes inline policy JSON to a temp file and returns its path.
func writePolicyFile(t *testing.T, inlinePolicy string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "policy-*.json")
	if err != nil {
		t.Fatalf("create temp policy file: %v", err)
	}
	if _, err := f.WriteString(inlinePolicy); err != nil {
		t.Fatalf("write policy file: %v", err)
	}
	f.Close()
	return f.Name()
}

// RunScenario executes the gromit-next binary for the given contract and returns the result.
func RunScenario(t *testing.T, c Contract, binary, fixtureBase string) *ScenarioResult {
	t.Helper()

	fixtureDir := filepath.Join(fixtureBase, c.Fixture)
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatalf("create fixture dir %s: %v", fixtureDir, err)
	}

	storeDir := filepath.Join(fixtureDir, c.StoreDir)
	if c.StoreDir == "" {
		storeDir = filepath.Join(fixtureDir, ".gromit-next")
	}

	specPath := filepath.Join(fixtureDir, c.Spec)

	args := []string{"exec", "spec",
		"--spec", specPath,
		"--project", c.Fixture,
		"--store-dir", storeDir,
	}

	// Determine policy path.
	// Policy paths are relative to fixtureBase (shared policies/ dir), not fixtureDir.
	policyPath := ""
	if c.InlinePolicy != "" {
		policyPath = writePolicyFile(t, c.InlinePolicy)
	} else if c.Policy != "" {
		policyPath = filepath.Join(fixtureBase, c.Policy)
	}
	if policyPath != "" {
		args = append(args, "--policy", policyPath)
	}

	args = append(args, c.ExtraFlags...)

	cmd := exec.Command(binary, args...)
	cmd.Dir = fixtureDir

	// Set a generous timeout via context — the test harness timeout handles this.
	var stdout strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	startTime := time.Now()
	t.Logf("running scenario %d: %s (started %s)", c.Scenario, c.Name, startTime.Format(time.RFC3339))
	t.Logf("  binary: %s", binary)
	t.Logf("  args: %v", args)
	t.Logf("  dir: %s", fixtureDir)

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Logf("scenario run error (non-exit): %v", err)
		}
	}

	t.Logf("  duration: %s, exit code: %d", time.Since(startTime).Round(time.Millisecond), exitCode)

	out := stdout.String()
	t.Logf("  stdout: %s", out)

	// Extract run ID from stdout.
	runID := ""
	if m := runIDRegex.FindStringSubmatch(out); len(m) == 2 {
		runID = m[1]
	}

	return &ScenarioResult{
		runID:      runID,
		storeDir:   storeDir,
		fixtureDir: fixtureDir,
		exitCode:   exitCode,
		stdout:     out,
	}
}

// RunContract resets the fixture and runs the full contract evaluation.
func RunContract(t *testing.T, c Contract, binary, fixtureBase string) {
	t.Helper()
	RequireE2E(t)

	fixtureDir := filepath.Join(fixtureBase, c.Fixture)
	ResetFixture(t, c, fixtureDir)

	result := RunScenario(t, c, binary, fixtureBase)
	evaluateAssertions(t, c, result)
}

// RunNamedContract finds a contract by scenario number and runs it.
func RunNamedContract(t *testing.T, scenario int, contractsDir, fixtureBase string) {
	t.Helper()
	RequireE2E(t)

	c := findContractByScenario(t, contractsDir, scenario)
	RunContract(t, c, e2eBinaryPath, fixtureBase)
}

// LoadContractByFile loads a single contract YAML file by filename from contractsDir.
func LoadContractByFile(t *testing.T, contractsDir, filename string) Contract {
	t.Helper()
	path := filepath.Join(contractsDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read contract file %s: %v", path, err)
	}
	var c Contract
	if err := yaml.Unmarshal(data, &c); err != nil {
		t.Fatalf("parse contract %s: %v", path, err)
	}
	return c
}

// RunContractByFile loads a specific YAML file by filename and runs it.
func RunContractByFile(t *testing.T, contractsDir, filename, fixtureBase string) {
	t.Helper()
	RequireE2E(t)
	c := LoadContractByFile(t, contractsDir, filename)
	RunContract(t, c, e2eBinaryPath, fixtureBase)
}

// --- Assertion evaluation ---

// evaluateAssertions checks all assertions in the contract against the scenario result.
func evaluateAssertions(t *testing.T, c Contract, result *ScenarioResult) {
	t.Helper()

	store := runstore.NewStore(result.storeDir)

	// Load RunState if we have a run ID.
	var rs *runstore.RunState
	if result.runID != "" {
		var err error
		rs, err = store.Get(result.runID)
		if err != nil {
			t.Errorf("load run state for %s: %v", result.runID, err)
		}
	}

	for i, a := range c.Assertions {
		label := fmt.Sprintf("assertion[%d]", i)
		checkAssertion(t, label, a, result, rs, store)
	}
}

// checkAssertion evaluates a single assertion.
func checkAssertion(t *testing.T, label string, a Assertion, result *ScenarioResult, rs *runstore.RunState, store *runstore.Store) {
	t.Helper()

	// --- Run state assertions ---

	if a.Status != "" {
		requireRunState(t, label+"/status", rs, result.runID)
		if rs != nil && rs.Status != a.Status {
			t.Errorf("%s: status = %q, want %q", label, rs.Status, a.Status)
		}
	}

	if len(a.StatusOneOf) > 0 {
		requireRunState(t, label+"/status_one_of", rs, result.runID)
		if rs != nil {
			found := false
			for _, s := range a.StatusOneOf {
				if rs.Status == s {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: status = %q, want one of %v", label, rs.Status, a.StatusOneOf)
			}
		}
	}

	if a.TerminalReason != "" {
		requireRunState(t, label+"/terminal_reason", rs, result.runID)
		if rs != nil && rs.TerminalReason != a.TerminalReason {
			t.Errorf("%s: terminal_reason = %q, want %q", label, rs.TerminalReason, a.TerminalReason)
		}
	}

	if a.FinalValidationPassed != nil {
		requireRunState(t, label+"/final_validation_passed", rs, result.runID)
		if rs != nil && rs.FinalValidationPassed != *a.FinalValidationPassed {
			t.Errorf("%s: final_validation_passed = %v, want %v", label, rs.FinalValidationPassed, *a.FinalValidationPassed)
		}
	}

	if a.FinalReviewPassed != nil {
		requireRunState(t, label+"/final_review_passed", rs, result.runID)
		if rs != nil && rs.FinalReviewPassed != *a.FinalReviewPassed {
			t.Errorf("%s: final_review_passed = %v, want %v", label, rs.FinalReviewPassed, *a.FinalReviewPassed)
		}
	}

	if a.FinalAcceptancePassed != nil {
		requireRunState(t, label+"/final_acceptance_passed", rs, result.runID)
		if rs != nil && rs.FinalAcceptancePassed != *a.FinalAcceptancePassed {
			t.Errorf("%s: final_acceptance_passed = %v, want %v", label, rs.FinalAcceptancePassed, *a.FinalAcceptancePassed)
		}
	}

	if a.CostUSDGt != nil {
		requireRunState(t, label+"/cost_usd_gt", rs, result.runID)
		if rs != nil && rs.AccumulatedCost <= *a.CostUSDGt {
			t.Errorf("%s: accumulated_cost = %.4f, want > %.4f", label, rs.AccumulatedCost, *a.CostUSDGt)
		}
	}

	if a.ReplansGte != nil {
		requireRunState(t, label+"/replans_gte", rs, result.runID)
		if rs != nil && rs.TotalReplans < *a.ReplansGte {
			t.Errorf("%s: total_replans = %d, want >= %d", label, rs.TotalReplans, *a.ReplansGte)
		}
	}

	if a.ReplansEq != nil {
		requireRunState(t, label+"/replans_eq", rs, result.runID)
		if rs != nil && rs.TotalReplans != *a.ReplansEq {
			t.Errorf("%s: total_replans = %d, want == %d", label, rs.TotalReplans, *a.ReplansEq)
		}
	}

	if a.CycleEq != nil {
		requireRunState(t, label+"/cycle_eq", rs, result.runID)
		if rs != nil && rs.Cycle != *a.CycleEq {
			t.Errorf("%s: cycle = %d, want == %d", label, rs.Cycle, *a.CycleEq)
		}
	}

	if a.EndedAtSet != nil {
		requireRunState(t, label+"/ended_at_set", rs, result.runID)
		if rs != nil {
			isSet := !rs.EndedAt.IsZero()
			if isSet != *a.EndedAtSet {
				t.Errorf("%s: ended_at_set = %v, want %v (ended_at = %v)", label, isSet, *a.EndedAtSet, rs.EndedAt)
			}
		}
	}

	// --- Evidence assertions ---

	if a.AcceptanceAllPass != nil {
		checkAcceptanceAllPass(t, label+"/acceptance_all_pass", result, *a.AcceptanceAllPass, store)
	}

	if a.ValidationPass != nil {
		checkValidationPass(t, label+"/validation_pass", result, *a.ValidationPass, store)
	}

	if a.NoErrorSeverityFindings != nil {
		checkNoErrorSeverityFindings(t, label+"/no_error_severity_findings", result, *a.NoErrorSeverityFindings, store)
	}

	if a.InvocationsCountGte != nil {
		checkInvocationsCountGte(t, label+"/invocations_count_gte", result, *a.InvocationsCountGte, store)
	}

	// --- Task assertions ---

	if a.AllTasksAttempted != nil {
		requireRunState(t, label+"/all_tasks_attempted", rs, result.runID)
		if rs != nil {
			allAttempted := true
			for _, task := range rs.Tasks {
				if task.Attempts == 0 {
					allAttempted = false
					break
				}
			}
			if allAttempted != *a.AllTasksAttempted {
				t.Errorf("%s: all_tasks_attempted = %v, want %v", label, allAttempted, *a.AllTasksAttempted)
			}
		}
	}

	if a.FilesChangedNonempty != nil {
		requireRunState(t, label+"/files_changed_nonempty", rs, result.runID)
		if rs != nil {
			hasAny := false
			for _, task := range rs.Tasks {
				if len(task.FilesChanged) > 0 {
					hasAny = true
					break
				}
			}
			if hasAny != *a.FilesChangedNonempty {
				t.Errorf("%s: files_changed_nonempty = %v, want %v", label, hasAny, *a.FilesChangedNonempty)
			}
		}
	}

	if a.FilesChangedNeverContains != "" {
		requireRunState(t, label+"/files_changed_never_contains", rs, result.runID)
		if rs != nil {
			for _, task := range rs.Tasks {
				for _, f := range task.FilesChanged {
					if strings.Contains(f, a.FilesChangedNeverContains) {
						t.Errorf("%s: files_changed contains %q in task %s (want never)", label, a.FilesChangedNeverContains, task.TaskID)
					}
				}
			}
		}
	}

	if a.AnyTaskFilesChangedContains != "" {
		requireRunState(t, label+"/any_task_files_changed_contains", rs, result.runID)
		if rs != nil {
			found := false
			for _, task := range rs.Tasks {
				for _, f := range task.FilesChanged {
					if strings.Contains(f, a.AnyTaskFilesChangedContains) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				t.Errorf("%s: no task has files_changed containing %q", label, a.AnyTaskFilesChangedContains)
			}
		}
	}

	// --- Filesystem assertions ---

	if a.FileContains != nil {
		path := a.FileContains.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(result.fixtureDir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: read file %s: %v", label, path, err)
		} else if !strings.Contains(string(data), a.FileContains.Pattern) {
			t.Errorf("%s: file %s does not contain %q", label, path, a.FileContains.Pattern)
		}
	}

	if a.FileNotModified != "" {
		// Check that file_not_modified file matches its git HEAD version.
		path := a.FileNotModified
		if !filepath.IsAbs(path) {
			path = filepath.Join(result.fixtureDir, path)
		}
		cmd := exec.Command("git", "diff", "--name-only", "HEAD", "--", a.FileNotModified)
		cmd.Dir = result.fixtureDir
		out, err := cmd.Output()
		if err != nil {
			t.Errorf("%s: git diff for file_not_modified %s: %v", label, a.FileNotModified, err)
		} else if strings.TrimSpace(string(out)) != "" {
			t.Errorf("%s: file_not_modified %s was modified", label, a.FileNotModified)
		}
	}

	// --- Event assertions ---

	if a.EventsContainType != "" {
		checkEventsContainType(t, label+"/events_contain_type", result, a.EventsContainType, store)
	}

	if a.EventsContainReplanSource != "" {
		checkEventsContainReplanSource(t, label+"/events_contain_replan_source", result, a.EventsContainReplanSource, store)
	}

	if a.EventsNotContainReplanSource != "" {
		checkEventsNotContainReplanSource(t, label+"/events_not_contain_replan_source", result, a.EventsNotContainReplanSource, store)
	}

	// --- CLI assertions ---

	if a.ExecShowContains != "" {
		out := runExecShow(t, label+"/exec_show_contains", result, false)
		if !strings.Contains(out, a.ExecShowContains) {
			t.Errorf("%s: exec show output does not contain %q\noutput:\n%s", label, a.ExecShowContains, out)
		}
	}

	if a.ExecShowNotContains != "" {
		out := runExecShow(t, label+"/exec_show_not_contains", result, false)
		if strings.Contains(out, a.ExecShowNotContains) {
			t.Errorf("%s: exec show output should not contain %q\noutput:\n%s", label, a.ExecShowNotContains, out)
		}
	}

	if a.ExecShowFullContains != "" {
		out := runExecShow(t, label+"/exec_show_full_contains", result, true)
		if !strings.Contains(out, a.ExecShowFullContains) {
			t.Errorf("%s: exec show --full output does not contain %q\noutput:\n%s", label, a.ExecShowFullContains, out)
		}
	}

	if a.ExecShowFullNotContains != "" {
		out := runExecShow(t, label+"/exec_show_full_not_contains", result, true)
		if strings.Contains(out, a.ExecShowFullNotContains) {
			t.Errorf("%s: exec show --full output should not contain %q\noutput:\n%s", label, a.ExecShowFullNotContains, out)
		}
	}

	if a.ExecListContains != "" {
		out := runExecList(t, label+"/exec_list_contains", result)
		if !strings.Contains(out, a.ExecListContains) {
			t.Errorf("%s: exec list output does not contain %q\noutput:\n%s", label, a.ExecListContains, out)
		}
	}

	if a.SpecListContains != "" {
		out := runSpecList(t, label+"/spec_list_contains", result)
		if !strings.Contains(out, a.SpecListContains) {
			t.Errorf("%s: spec list output does not contain %q\noutput:\n%s", label, a.SpecListContains, out)
		}
	}
}

// requireRunState logs an error if rs is nil (run ID was not found in output).
func requireRunState(t *testing.T, label string, rs *runstore.RunState, runID string) {
	t.Helper()
	if rs == nil {
		t.Errorf("%s: cannot check — run state not loaded (runID=%q)", label, runID)
	}
}

// --- Evidence helpers ---

// checkAcceptanceAllPass reads acceptance.json and checks all_pass.
func checkAcceptanceAllPass(t *testing.T, label string, result *ScenarioResult, want bool, store *runstore.Store) {
	t.Helper()
	if result.runID == "" {
		t.Errorf("%s: no run ID to check acceptance", label)
		return
	}
	path := filepath.Join(store.RunEvidenceDir(result.runID), "acceptance.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: read acceptance.json: %v", label, err)
		return
	}
	var parsed struct {
		AllPass bool `json:"all_pass"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("%s: parse acceptance.json: %v", label, err)
		return
	}
	if parsed.AllPass != want {
		t.Errorf("%s: acceptance all_pass = %v, want %v", label, parsed.AllPass, want)
	}
}

// checkValidationPass reads validation.json and checks pass.
func checkValidationPass(t *testing.T, label string, result *ScenarioResult, want bool, store *runstore.Store) {
	t.Helper()
	if result.runID == "" {
		t.Errorf("%s: no run ID to check validation", label)
		return
	}
	path := filepath.Join(store.RunEvidenceDir(result.runID), "validation.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: read validation.json: %v", label, err)
		return
	}
	var parsed struct {
		Pass bool `json:"pass"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("%s: parse validation.json: %v", label, err)
		return
	}
	if parsed.Pass != want {
		t.Errorf("%s: validation pass = %v, want %v", label, parsed.Pass, want)
	}
}

// checkNoErrorSeverityFindings reads review.json and checks for error-severity findings.
func checkNoErrorSeverityFindings(t *testing.T, label string, result *ScenarioResult, wantNoErrors bool, store *runstore.Store) {
	t.Helper()
	if result.runID == "" {
		t.Errorf("%s: no run ID to check review findings", label)
		return
	}
	path := filepath.Join(store.RunEvidenceDir(result.runID), "review.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No review.json means no findings — treat as no errors.
			if !wantNoErrors {
				t.Errorf("%s: review.json not found but wanted error findings", label)
			}
			return
		}
		t.Errorf("%s: read review.json: %v", label, err)
		return
	}

	// review.json is a map[string][]Finding where Finding has "severity" as a string.
	var parsed map[string][]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("%s: parse review.json: %v", label, err)
		return
	}

	hasError := false
	for _, findings := range parsed {
		for _, raw := range findings {
			var finding struct {
				Severity string `json:"severity"`
			}
			if err := json.Unmarshal(raw, &finding); err != nil {
				continue
			}
			if finding.Severity == "error" || finding.Severity == "critical" {
				hasError = true
				break
			}
		}
		if hasError {
			break
		}
	}

	if wantNoErrors && hasError {
		t.Errorf("%s: found error-severity findings in review.json but want none", label)
	}
	if !wantNoErrors && !hasError {
		t.Errorf("%s: want error-severity findings in review.json but found none", label)
	}
}

// checkInvocationsCountGte reads metrics.json and checks the invocation count.
func checkInvocationsCountGte(t *testing.T, label string, result *ScenarioResult, want int, store *runstore.Store) {
	t.Helper()
	if result.runID == "" {
		t.Errorf("%s: no run ID to check invocations", label)
		return
	}
	path := filepath.Join(store.RunEvidenceDir(result.runID), "metrics.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: read metrics.json: %v", label, err)
		return
	}
	var parsed struct {
		Invocations []json.RawMessage `json:"invocations"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("%s: parse metrics.json: %v", label, err)
		return
	}
	count := len(parsed.Invocations)
	if count < want {
		t.Errorf("%s: invocations count = %d, want >= %d", label, count, want)
	}
}

// checkEventsContainType reads events.jsonl and checks for a specific event type.
func checkEventsContainType(t *testing.T, label string, result *ScenarioResult, eventType string, store *runstore.Store) {
	t.Helper()
	if result.runID == "" {
		t.Errorf("%s: no run ID to check events", label)
		return
	}
	eventsPath := filepath.Join(store.RunDir(result.runID), "events.jsonl")
	f, err := os.Open(eventsPath)
	if err != nil {
		t.Errorf("%s: open events.jsonl: %v", label, err)
		return
	}
	defer f.Close()

	found := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Type == eventType {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("%s: events.jsonl does not contain event type %q", label, eventType)
	}
}

// checkEventsContainReplanSource reads events.jsonl and checks for a
// replan_triggered event with the given source field value.
func checkEventsContainReplanSource(t *testing.T, label string, result *ScenarioResult, source string, store *runstore.Store) {
	t.Helper()
	if result.runID == "" {
		t.Errorf("%s: no run ID to check events", label)
		return
	}
	eventsPath := filepath.Join(store.RunDir(result.runID), "events.jsonl")
	f, err := os.Open(eventsPath)
	if err != nil {
		t.Errorf("%s: open events.jsonl: %v", label, err)
		return
	}
	defer f.Close()

	found := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev struct {
			Type   string `json:"type"`
			Source string `json:"source"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Type == "replan_triggered" && ev.Source == source {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("%s: events.jsonl does not contain replan_triggered event with source %q", label, source)
	}
}

// checkEventsNotContainReplanSource reads events.jsonl and fails if any
// replan_triggered event has the given source field value.
func checkEventsNotContainReplanSource(t *testing.T, label string, result *ScenarioResult, source string, store *runstore.Store) {
	t.Helper()
	if result.runID == "" {
		t.Errorf("%s: no run ID to check events", label)
		return
	}
	eventsPath := filepath.Join(store.RunDir(result.runID), "events.jsonl")
	f, err := os.Open(eventsPath)
	if err != nil {
		t.Errorf("%s: open events.jsonl: %v", label, err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev struct {
			Type   string `json:"type"`
			Source string `json:"source"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Type == "replan_triggered" && ev.Source == source {
			t.Errorf("%s: events.jsonl contains unexpected replan_triggered event with source %q", label, source)
			return
		}
	}
}

// --- CLI helpers ---

// gromitNextBinary returns the path of the binary built during the test, stored
// in a package-level variable set by the harness test.
// For CLI assertions we re-invoke the binary rather than calling internal functions
// (which live in package main and cannot be imported).

// runExecShow runs `gromit-next exec show <runID> [--full] --store-dir <storeDir>`
// and returns its output. Binary is located by finding the built binary via
// the PATH or via the e2e fixture convention.
func runExecShow(t *testing.T, label string, result *ScenarioResult, full bool) string {
	t.Helper()
	if result.runID == "" {
		t.Errorf("%s: no run ID available for exec show", label)
		return ""
	}
	binary := findBinary(t)
	args := []string{"exec", "show", result.runID, "--store-dir", result.storeDir}
	if full {
		args = append(args, "--full")
	}
	cmd := exec.Command(binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("%s: exec show stderr: %s", label, exitErr.Stderr)
		}
		t.Errorf("%s: exec show failed: %v", label, err)
		return ""
	}
	return string(out)
}

// runExecList runs `gromit-next exec list --project <project> --store-dir <storeDir>`
// and returns its output.
func runExecList(t *testing.T, label string, result *ScenarioResult) string {
	t.Helper()
	binary := findBinary(t)
	// Extract project from the fixture path (last path component).
	project := filepath.Base(result.fixtureDir)
	args := []string{"exec", "list", "--project", project, "--store-dir", result.storeDir}
	cmd := exec.Command(binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("%s: exec list stderr: %s", label, exitErr.Stderr)
		}
		t.Errorf("%s: exec list failed: %v", label, err)
		return ""
	}
	return string(out)
}

// runSpecList runs `gromit-next spec list --project <project> --store-dir <storeDir> --specs-dir <specsDir>`
// and returns its output.
func runSpecList(t *testing.T, label string, result *ScenarioResult) string {
	t.Helper()
	binary := findBinary(t)
	project := filepath.Base(result.fixtureDir)
	// The specs dir is the fixture dir itself (specs are .md files at top level or in a subdir).
	// Use the fixture dir as specs-dir so we don't need workspace resolution.
	args := []string{"spec", "list",
		"--project", project,
		"--store-dir", result.storeDir,
		"--specs-dir", filepath.Join(result.fixtureDir, "specs"),
	}
	cmd := exec.Command(binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("%s: spec list stderr: %s", label, exitErr.Stderr)
		}
		t.Errorf("%s: spec list failed: %v", label, err)
		return ""
	}
	return string(out)
}

// e2eBinaryPath is set by SetBinaryPath and reused for CLI assertions.
// It is stored as a package variable so CLI helper functions can access it without
// threading the path through every call.
var e2eBinaryPath string

// SetBinaryPath stores the built binary path for use in CLI assertion helpers.
func SetBinaryPath(path string) {
	e2eBinaryPath = path
}

// findBinary returns the cached binary path, failing the test if it hasn't been set.
func findBinary(t *testing.T) string {
	t.Helper()
	if e2eBinaryPath == "" {
		t.Fatal("e2eBinaryPath not set — call SetBinaryPath first")
	}
	return e2eBinaryPath
}
