package tdd

import (
	"fmt"
	"testing"
)

func fakeReadFile(contents map[string]string) ReadFileFn {
	return func(path string) (string, error) {
		content, ok := contents[path]
		if !ok {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return content, nil
	}
}

func fakeGetDiff() GetDiffFn {
	return func() (string, error) {
		return "", nil
	}
}

func TestClassifyTouchedFilesSeparatesTestFromImpl(t *testing.T) {
	paths := []string{
		"internal/foo/bar.go",
		"internal/foo/bar_test.go",
		"internal/baz/qux.go",
	}

	testFiles, implFiles := ClassifyTouchedFiles(paths)

	if len(testFiles) != 1 {
		t.Fatalf("expected 1 test file, got %d", len(testFiles))
	}
	if testFiles[0] != "internal/foo/bar_test.go" {
		t.Fatalf("expected bar_test.go, got %s", testFiles[0])
	}
	if len(implFiles) != 2 {
		t.Fatalf("expected 2 impl files, got %d", len(implFiles))
	}
	if implFiles[0] != "internal/foo/bar.go" {
		t.Fatalf("expected bar.go first, got %s", implFiles[0])
	}
	if implFiles[1] != "internal/baz/qux.go" {
		t.Fatalf("expected qux.go second, got %s", implFiles[1])
	}
}

func TestAssembleRedHandoffFirstCycleReturnsEmptyMapsAndSpecExcerpt(t *testing.T) {
	state := CycleState{
		CycleNumber:  1,
		MaxCycles:    10,
		Remaining:    []string{"users can log in with valid credentials"},
		TouchedFiles: []string{},
	}

	readFile := fakeReadFile(map[string]string{})
	getDiff := fakeGetDiff()

	handoff, err := AssembleRedHandoff(state, readFile, getDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if handoff.SpecExcerpt != "users can log in with valid credentials" {
		t.Fatalf("expected spec excerpt from Remaining[0], got %q", handoff.SpecExcerpt)
	}
	if len(handoff.TestFiles) != 0 {
		t.Fatalf("expected empty test files on first cycle, got %d", len(handoff.TestFiles))
	}
	if len(handoff.ImplFiles) != 0 {
		t.Fatalf("expected empty impl files on first cycle, got %d", len(handoff.ImplFiles))
	}
}

func TestAssembleRedHandoffReadsExistingFilesOnSubsequentCycles(t *testing.T) {
	state := CycleState{
		CycleNumber: 2,
		MaxCycles:   10,
		Remaining:   []string{"users can reset password"},
		TouchedFiles: []string{
			"internal/auth/login_test.go",
			"internal/auth/login.go",
		},
	}

	readFile := fakeReadFile(map[string]string{
		"internal/auth/login_test.go": "package auth\n\nfunc TestLogin(t *testing.T) {}",
		"internal/auth/login.go":      "package auth\n\nfunc Login() {}",
	})
	getDiff := fakeGetDiff()

	handoff, err := AssembleRedHandoff(state, readFile, getDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if handoff.SpecExcerpt != "users can reset password" {
		t.Fatalf("expected next remaining spec, got %q", handoff.SpecExcerpt)
	}
	if len(handoff.TestFiles) != 1 {
		t.Fatalf("expected 1 test file, got %d", len(handoff.TestFiles))
	}
	if handoff.TestFiles["internal/auth/login_test.go"] == "" {
		t.Fatalf("expected test file content to be populated")
	}
	if len(handoff.ImplFiles) != 1 {
		t.Fatalf("expected 1 impl file, got %d", len(handoff.ImplFiles))
	}
	if handoff.ImplFiles["internal/auth/login.go"] == "" {
		t.Fatalf("expected impl file content to be populated")
	}
}

func TestAssembleGreenHandoffCapturesTestContentAndFailure(t *testing.T) {
	touchedFiles := []string{
		"internal/auth/login_test.go",
		"internal/auth/login.go",
	}
	testOutput := "--- FAIL: TestLogin (0.00s)\n    login_test.go:10: expected true, got false"

	readFile := fakeReadFile(map[string]string{
		"internal/auth/login_test.go": "package auth\n\nfunc TestLogin(t *testing.T) { t.Fatal(\"expected true\") }",
		"internal/auth/login.go":      "package auth\n\nfunc Login() bool { return false }",
	})

	handoff, err := AssembleGreenHandoff(testOutput, readFile, touchedFiles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if handoff.TestFailureOutput != testOutput {
		t.Fatalf("expected test failure output to be preserved, got %q", handoff.TestFailureOutput)
	}
	if len(handoff.ImplFiles) != 1 {
		t.Fatalf("expected 1 impl file, got %d", len(handoff.ImplFiles))
	}
	if handoff.ImplFiles["internal/auth/login.go"] == "" {
		t.Fatalf("expected impl file content to be populated")
	}
	if handoff.FailingTest == "" {
		t.Fatalf("expected failing test content to be populated")
	}
}

func TestAssembleRefactorHandoffReadsAllTouchedFiles(t *testing.T) {
	touchedFiles := []string{
		"internal/auth/login_test.go",
		"internal/auth/login.go",
		"internal/auth/session.go",
	}

	readFile := fakeReadFile(map[string]string{
		"internal/auth/login_test.go": "package auth\n\nfunc TestLogin(t *testing.T) {}",
		"internal/auth/login.go":      "package auth\n\nfunc Login() {}",
		"internal/auth/session.go":    "package auth\n\nfunc Session() {}",
	})

	handoff, err := AssembleRefactorHandoff(readFile, touchedFiles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(handoff.TestFiles) != 1 {
		t.Fatalf("expected 1 test file, got %d", len(handoff.TestFiles))
	}
	if handoff.TestFiles["internal/auth/login_test.go"] == "" {
		t.Fatalf("expected test file content to be populated")
	}
	if len(handoff.ImplFiles) != 2 {
		t.Fatalf("expected 2 impl files, got %d", len(handoff.ImplFiles))
	}
	if handoff.ImplFiles["internal/auth/login.go"] == "" {
		t.Fatalf("expected login.go content to be populated")
	}
	if handoff.ImplFiles["internal/auth/session.go"] == "" {
		t.Fatalf("expected session.go content to be populated")
	}
}

func TestAssembleCycleStateIncrementsCycleAndMovesFirstRemainingToCovered(t *testing.T) {
	prev := CycleState{
		CycleNumber:  1,
		MaxCycles:    10,
		CoveredSoFar: []string{},
		Remaining:    []string{"req A", "req B", "req C"},
		TouchedFiles: []string{"a.go"},
	}

	next := AssembleCycleState(prev, "some red phase output")

	if next.CycleNumber != 2 {
		t.Fatalf("expected CycleNumber 2, got %d", next.CycleNumber)
	}
	if next.MaxCycles != 10 {
		t.Fatalf("expected MaxCycles preserved at 10, got %d", next.MaxCycles)
	}
	if len(next.CoveredSoFar) != 1 || next.CoveredSoFar[0] != "req A" {
		t.Fatalf("expected CoveredSoFar to contain 'req A', got %v", next.CoveredSoFar)
	}
	if len(next.Remaining) != 2 || next.Remaining[0] != "req B" {
		t.Fatalf("expected Remaining to start with 'req B', got %v", next.Remaining)
	}
	if len(next.TouchedFiles) != 1 || next.TouchedFiles[0] != "a.go" {
		t.Fatalf("expected TouchedFiles preserved, got %v", next.TouchedFiles)
	}
}

func TestAssembleCycleStateSetsDoneWhenLastRemainingConsumed(t *testing.T) {
	prev := CycleState{
		CycleNumber:  3,
		MaxCycles:    10,
		CoveredSoFar: []string{"req A", "req B"},
		Remaining:    []string{"req C"},
		TouchedFiles: []string{"a.go"},
	}

	next := AssembleCycleState(prev, "output")

	if len(next.Remaining) != 0 {
		t.Fatalf("expected empty Remaining, got %v", next.Remaining)
	}
	if len(next.CoveredSoFar) != 3 {
		t.Fatalf("expected 3 covered items, got %d", len(next.CoveredSoFar))
	}
	if !next.Done {
		t.Fatalf("expected Done=true when all requirements covered")
	}
}
