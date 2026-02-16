package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type consolidatedSmokeMatrixEntry struct {
	CaseID      string
	Decision    string
	Rationale   string
	Destination string
}

var consolidatedSmokeFuncRe = regexp.MustCompile(`^func (Test[[:alnum:]_]+)\(`)

func loadConsolidatedSmokeMatrix(t *testing.T, projectRoot string) map[string]consolidatedSmokeMatrixEntry {
	t.Helper()

	matrixPath := filepath.Join(projectRoot, "docs", "smoke_coverage_matrix.md")
	data, err := os.Open(matrixPath)
	if err != nil {
		t.Fatalf("open consolidated smoke coverage matrix: %v", err)
	}
	defer func() {
		if closeErr := data.Close(); closeErr != nil {
			t.Fatalf("close consolidated smoke coverage matrix: %v", closeErr)
		}
	}()

	entries := make(map[string]consolidatedSmokeMatrixEntry)
	scanner := bufio.NewScanner(data)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.Contains(line, "---") {
			continue
		}

		fields := splitMarkdownTableLine(line)
		if len(fields) != 4 {
			continue
		}
		caseID := fields[0]
		if caseID == "case" || caseID == "Case" {
			continue
		}

		entry := consolidatedSmokeMatrixEntry{
			CaseID:      caseID,
			Decision:    fields[1],
			Rationale:   fields[2],
			Destination: fields[3],
		}

		if _, exists := entries[caseID]; exists {
			t.Fatalf("duplicate smoke matrix case %s", caseID)
		}
		entries[caseID] = entry
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan consolidated smoke coverage matrix: %v", err)
	}

	return entries
}

func splitMarkdownTableLine(line string) []string {
	parts := strings.Split(line, "|")
	fields := make([]string, 0, 4)
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		fields = append(fields, trimmed)
	}
	return fields
}

func collectAcceptanceTests(t *testing.T, projectRoot string, files []string) []string {
	t.Helper()

	cases := make([]string, 0)
	for _, rel := range files {
		path := filepath.Join(projectRoot, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}

		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			if m := consolidatedSmokeFuncRe.FindStringSubmatch(line); m != nil {
				cases = append(cases, rel+":"+strings.TrimSpace(m[1]))
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("scan %s: %v", rel, err)
		}
	}

	return cases
}

func listPackageTests(t *testing.T, projectRoot, pkg string) string {
	t.Helper()

	cmd := exec.Command("go", "test", pkg, "-list", "Test")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -list %s failed: %v\n%s", pkg, err, string(out))
	}

	return string(out)
}

func TestSmokeCoverageMatrix_ConsolidatedFileHasHeader(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	matrixPath := filepath.Join(projectRoot, "docs", "smoke_coverage_matrix.md")
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("read consolidated smoke matrix: %v", err)
	}

	header := "| case | decision | rationale | destination |"
	if !strings.Contains(string(data), header) {
		t.Fatalf("consolidated smoke matrix missing header %q", header)
	}
}

func TestSmokeCoverageMatrix_ConsolidatedCaseMappingIsComplete(t *testing.T) {
	// Expected failure: Consolidated smoke coverage matrix file and
	// BuildConsolidatedSmokeMatrix generator do not exist yet.
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	cmdFiles := []string{
		"cmd/gromit/debug_agent_acceptance_test.go",
		"cmd/gromit/explore_codex_help_acceptance_test.go",
		"cmd/gromit/review_spec_validation_acceptance_test.go",
	}
	runnerFiles := []string{
		"internal/runner/validation_extraction_acceptance_test.go",
		"internal/runner/invocation_timeout_acceptance_test.go",
		"internal/runner/worktree_merge_acceptance_test.go",
	}

	cases := append(
		collectAcceptanceTests(t, projectRoot, cmdFiles),
		collectAcceptanceTests(t, projectRoot, runnerFiles)...,
	)
	entries := loadConsolidatedSmokeMatrix(t, projectRoot)

	for _, caseID := range cases {
		entry, ok := entries[caseID]
		if !ok {
			t.Fatalf("consolidated smoke matrix missing case %s", caseID)
		}
		if entry.Decision != "keep" && entry.Decision != "move" {
			t.Fatalf("%s has invalid decision %q", caseID, entry.Decision)
		}
		if entry.Rationale == "" || entry.Rationale == "-" {
			t.Fatalf("%s has empty rationale", caseID)
		}
	}

	for caseID := range entries {
		found := false
		for _, known := range cases {
			if caseID == known {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("consolidated smoke matrix includes unknown case %s", caseID)
		}
	}
}

func TestSmokeCoverageMatrix_KeepSetIsMinimalHighValue(t *testing.T) {
	// Expected failure: Consolidated keep set is not limited to the critical
	// E2E smoke cases defined by ConsolidatedSmokeKeepSet yet.
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	entries := loadConsolidatedSmokeMatrix(t, projectRoot)

	expectedKeep := map[string]bool{
		"cmd/gromit/debug_agent_acceptance_test.go:TestCmdSmoke_DebugAgentResolutionEndToEnd":                  true,
		"cmd/gromit/explore_codex_help_acceptance_test.go:TestCmdSmoke_ExploreAgentSelectionEndToEnd":          true,
		"cmd/gromit/review_spec_validation_acceptance_test.go:TestCmdSmoke_ReviewSpecValidationEndToEnd":       true,
		"internal/runner/validation_extraction_acceptance_test.go:TestRunnerSmoke_RunSingleBeadHappyPath":      true,
		"internal/runner/invocation_timeout_acceptance_test.go:TestRunnerSmoke_ValidationFailureEscalatesTier": true,
		"internal/runner/worktree_merge_acceptance_test.go:TestRunnerSmoke_WorktreeMergeModesEndToEnd":         true,
	}

	actualKeep := make(map[string]bool)
	for caseID, entry := range entries {
		if entry.Decision == "keep" {
			actualKeep[caseID] = true
		}
	}

	if len(actualKeep) != len(expectedKeep) {
		t.Fatalf("keep set size=%d, want=%d (actual=%v)", len(actualKeep), len(expectedKeep), actualKeep)
	}

	for caseID := range expectedKeep {
		if !actualKeep[caseID] {
			t.Fatalf("keep set missing critical smoke case %s", caseID)
		}
	}
}

func TestSmokeCoverageMatrix_MoveCasesHaveConcreteUnitDestinations(t *testing.T) {
	// Expected failure: Consolidated move destinations are not validated against
	// unit test suites, and ConsolidatedMoveDestinationResolution is missing.
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	entries := loadConsolidatedSmokeMatrix(t, projectRoot)
	cmdUnitTests := listPackageTests(t, projectRoot, "./cmd/gromit")
	runnerUnitTests := listPackageTests(t, projectRoot, "./internal/runner")

	for caseID, entry := range entries {
		if entry.Decision != "move" {
			continue
		}
		if entry.Destination == "" || entry.Destination == "-" {
			t.Fatalf("%s has empty move destination", caseID)
		}

		parts := strings.Split(entry.Destination, ":")
		if len(parts) != 2 {
			t.Fatalf("%s destination %q must be file:suite", caseID, entry.Destination)
		}
		file := parts[0]
		suite := parts[1]
		if !strings.HasSuffix(file, "_test.go") {
			t.Fatalf("%s destination file must be *_test.go, got %q", caseID, file)
		}

		var suiteList string
		switch {
		case strings.HasPrefix(file, "cmd/gromit/"):
			suiteList = cmdUnitTests
		case strings.HasPrefix(file, "internal/runner/"):
			suiteList = runnerUnitTests
		default:
			t.Fatalf("%s destination %q must point to cmd/gromit or internal/runner test suites", caseID, file)
		}

		if !strings.Contains(suiteList, suite) {
			t.Fatalf("%s destination suite %s not found in %s", caseID, suite, file)
		}
	}
}
