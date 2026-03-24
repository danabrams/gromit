package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestFilesInDiff_WithValidDiffLine(t *testing.T) {
	// AC 1: Returns map containing 'validate.go' key when diff has '+++ b/internal/next/specloop/stages/validate.go' line
	diffText := "+++ b/internal/next/specloop/stages/validate.go"
	result := FilesInDiff(diffText)

	if !result["internal/next/specloop/stages/validate.go"] {
		t.Errorf("expected 'internal/next/specloop/stages/validate.go' in result, got %v", result)
	}

	// Verify it contains exactly one file
	if len(result) != 1 {
		t.Errorf("expected 1 file in result, got %d", len(result))
	}
}

func TestFilesInDiff_EmptyString(t *testing.T) {
	// AC 2: Returns empty map for empty string
	result := FilesInDiff("")

	if len(result) != 0 {
		t.Errorf("expected empty map for empty string, got %v", result)
	}
}

func TestFilesInDiff_NoDiffLines(t *testing.T) {
	// AC 3: Returns empty map for diff with no +++ b/ lines
	diffText := "--- a/some/file\n@@ -1,5 +1,6 @@\n normal line\n another line"
	result := FilesInDiff(diffText)

	if len(result) != 0 {
		t.Errorf("expected empty map for diff with no '+++ b/' lines, got %v", result)
	}
}

func TestFilesInDiff_StripsPrefix(t *testing.T) {
	// AC 4: Strips b/ prefix correctly
	diffText := "+++ b/cmd/main.go\n+++ b/internal/helper.go"
	result := FilesInDiff(diffText)

	// Should not contain "b/" prefix
	if result["b/cmd/main.go"] {
		t.Errorf("expected 'b/' prefix to be stripped, but found it in result")
	}

	if !result["cmd/main.go"] || !result["internal/helper.go"] {
		t.Errorf("expected 'cmd/main.go' and 'internal/helper.go' in result, got %v", result)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 files in result, got %d", len(result))
	}
}

func TestFilesInDiff_MultipleDiffLines(t *testing.T) {
	diffText := "diff --git a/file1.go b/file1.go\n+++ b/file1.go\ndiff --git a/file2.go b/file2.go\n+++ b/file2.go"
	result := FilesInDiff(diffText)

	if !result["file1.go"] || !result["file2.go"] {
		t.Errorf("expected 'file1.go' and 'file2.go' in result, got %v", result)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 files in result, got %d", len(result))
	}
}

func TestFilesInDiff_IgnoresNonDiffLines(t *testing.T) {
	diffText := "++ b/not/a/real/diff\n+++ b/valid/file.go\n+++ b/another/valid.go"
	result := FilesInDiff(diffText)

	// Single '+' should not match, only '+++'
	if result["not/a/real/diff"] {
		t.Errorf("expected lines starting with '++ b/' to be ignored")
	}

	if !result["valid/file.go"] || !result["another/valid.go"] {
		t.Errorf("expected valid diff lines to be parsed, got %v", result)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 files in result, got %d", len(result))
	}
}

func TestFilesInDiff_DuplicateFiles(t *testing.T) {
	// Same file appearing multiple times should result in single entry
	diffText := "+++ b/file.go\n+++ b/file.go\n+++ b/file.go"
	result := FilesInDiff(diffText)

	if !result["file.go"] {
		t.Errorf("expected 'file.go' in result")
	}

	// The value should be true (map entries are always present if key exists)
	if len(result) != 1 {
		t.Errorf("expected 1 unique file in result, got %d", len(result))
	}
}

func TestFilesInDiff_EmptyDiffLine(t *testing.T) {
	// Edge case: "+++ b/" with no filename
	diffText := "+++ b/\n+++ b/valid/file.go"
	result := FilesInDiff(diffText)

	// Empty filename should be skipped
	if result[""] {
		t.Errorf("expected empty filename to be excluded from result")
	}

	if !result["valid/file.go"] {
		t.Errorf("expected 'valid/file.go' in result, got %v", result)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 file in result, got %d", len(result))
	}
}

// mockVerifier implements FindingVerifier for testing.
// It records calls and returns predetermined dispositions based on the finding's file path.
type mockVerifier struct {
	dispositions map[string]VerifierDisposition
	errors       map[string]error
	callCount    int
}

func (m *mockVerifier) Verify(ctx context.Context, f Finding, workDir string) (VerifierResult, error) {
	m.callCount++
	if err, ok := m.errors[f.File]; ok {
		return VerifierResult{}, err
	}
	disposition, ok := m.dispositions[f.File]
	if !ok {
		disposition = DispositionConfirmed
	}
	return VerifierResult{
		Finding:     f,
		Disposition: disposition,
	}, nil
}

func TestVerifyBlockingFindings_InDiffPassthrough(t *testing.T) {
	// AC 2: in-diff finding passes to kept without calling verifier and results is empty
	ctx := context.Background()
	verifier := &mockVerifier{
		dispositions: map[string]VerifierDisposition{},
	}

	findings := []Finding{
		{
			Facet:       "test",
			Severity:    SeverityError,
			File:        "modified.go",
			Line:        10,
			Description: "test finding",
			Cycle:       1,
		},
	}
	diffFiles := map[string]bool{"modified.go": true}

	kept, results := VerifyBlockingFindings(ctx, findings, diffFiles, verifier, "/test")

	if len(kept) != 1 {
		t.Errorf("expected 1 kept finding, got %d", len(kept))
	}
	if kept[0].File != "modified.go" {
		t.Errorf("expected kept finding to be 'modified.go', got %q", kept[0].File)
	}
	if verifier.callCount != 0 {
		t.Errorf("expected verifier not to be called for in-diff finding, but was called %d times", verifier.callCount)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results for in-diff findings, got %d results", len(results))
	}
}

func TestVerifyBlockingFindings_ConfirmedRetained(t *testing.T) {
	// AC 3: out-of-diff confirmed finding stays in kept with original severity
	ctx := context.Background()
	verifier := &mockVerifier{
		dispositions: map[string]VerifierDisposition{
			"unmodified.go": DispositionConfirmed,
		},
	}

	originalSeverity := SeverityError
	findings := []Finding{
		{
			Facet:       "test",
			Severity:    originalSeverity,
			File:        "unmodified.go",
			Line:        20,
			Description: "test finding",
			Cycle:       1,
		},
	}
	diffFiles := map[string]bool{} // Empty diff

	kept, results := VerifyBlockingFindings(ctx, findings, diffFiles, verifier, "/test")

	if len(kept) != 1 {
		t.Errorf("expected 1 kept finding, got %d", len(kept))
	}
	if kept[0].File != "unmodified.go" {
		t.Errorf("expected kept finding to be 'unmodified.go', got %q", kept[0].File)
	}
	if kept[0].Severity != originalSeverity {
		t.Errorf("expected original severity %q, got %q", originalSeverity.String(), kept[0].Severity.String())
	}
	if len(results) != 1 {
		t.Errorf("expected 1 verification result, got %d", len(results))
	}
}

func TestVerifyBlockingFindings_Fixed(t *testing.T) {
	// AC 4: out-of-diff fixed finding is absent from kept
	ctx := context.Background()
	verifier := &mockVerifier{
		dispositions: map[string]VerifierDisposition{
			"unmodified.go": DispositionFixed,
		},
	}

	findings := []Finding{
		{
			Facet:       "test",
			Severity:    SeverityError,
			File:        "unmodified.go",
			Line:        30,
			Description: "test finding",
			Cycle:       1,
		},
	}
	diffFiles := map[string]bool{} // Empty diff

	kept, results := VerifyBlockingFindings(ctx, findings, diffFiles, verifier, "/test")

	if len(kept) != 0 {
		t.Errorf("expected 0 kept findings for fixed finding, got %d", len(kept))
	}
	if len(results) != 1 {
		t.Errorf("expected 1 verification result, got %d", len(results))
	}
	if results[0].Disposition != DispositionFixed {
		t.Errorf("expected fixed disposition, got %q", results[0].Disposition)
	}
}

func TestVerifyBlockingFindings_Downgraded(t *testing.T) {
	// AC 5: out-of-diff downgraded finding stays in kept with Severity=SeverityWarning
	ctx := context.Background()
	verifier := &mockVerifier{
		dispositions: map[string]VerifierDisposition{
			"unmodified.go": DispositionDowngraded,
		},
	}

	findings := []Finding{
		{
			Facet:       "test",
			Severity:    SeverityError,
			File:        "unmodified.go",
			Line:        40,
			Description: "test finding",
			Cycle:       1,
		},
	}
	diffFiles := map[string]bool{} // Empty diff

	kept, results := VerifyBlockingFindings(ctx, findings, diffFiles, verifier, "/test")

	if len(kept) != 1 {
		t.Errorf("expected 1 kept finding, got %d", len(kept))
	}
	if kept[0].File != "unmodified.go" {
		t.Errorf("expected kept finding to be 'unmodified.go', got %q", kept[0].File)
	}
	if kept[0].Severity != SeverityWarning {
		t.Errorf("expected downgraded severity to be warning, got %q", kept[0].Severity.String())
	}
	if len(results) != 1 {
		t.Errorf("expected 1 verification result, got %d", len(results))
	}
	if results[0].Disposition != DispositionDowngraded {
		t.Errorf("expected downgraded disposition, got %q", results[0].Disposition)
	}
}

func TestVerifyBlockingFindings_VerifierError(t *testing.T) {
	// When verifier returns an error, findings are retained with fail-safe DispositionConfirmed
	ctx := context.Background()
	verifier := &mockVerifier{
		dispositions: map[string]VerifierDisposition{},
		errors: map[string]error{
			"unmodified.go": os.ErrPermission,
		},
	}

	findings := []Finding{
		{
			Facet:       "test",
			Severity:    SeverityError,
			File:        "unmodified.go",
			Line:        50,
			Description: "test finding",
			Cycle:       1,
		},
	}
	diffFiles := map[string]bool{} // Empty diff

	kept, results := VerifyBlockingFindings(ctx, findings, diffFiles, verifier, "/test")

	if len(kept) != 1 {
		t.Errorf("expected 1 kept finding on verifier error, got %d", len(kept))
	}
	if kept[0].File != "unmodified.go" {
		t.Errorf("expected kept finding to be 'unmodified.go', got %q", kept[0].File)
	}
	if kept[0].Severity != SeverityError {
		t.Errorf("expected original severity preserved, got %q", kept[0].Severity.String())
	}
	if len(results) != 1 {
		t.Errorf("expected 1 verification result, got %d", len(results))
	}
	if results[0].Disposition != DispositionConfirmed {
		t.Errorf("expected confirmed disposition on error, got %q", results[0].Disposition)
	}
	if !strings.Contains(results[0].Reason, "verification error") {
		t.Errorf("expected reason to mention verification error, got %q", results[0].Reason)
	}
}

func TestLLMFindingVerifier_FileUnreadable(t *testing.T) {
	// AC 6: Verify returns DispositionConfirmed with reason containing 'file unreadable' when file does not exist
	ctx := context.Background()

	invoker := &mockInvoker{
		result: &provider.Result{
			Output: "confirmed",
		},
	}

	verifier := NewLLMFindingVerifier(invoker)

	finding := Finding{
		Facet:       "test",
		Severity:    SeverityError,
		File:        "nonexistent.go",
		Line:        10,
		Description: "test finding",
		Cycle:       1,
	}

	result, _ := verifier.Verify(ctx, finding, "/nonexistent/path/that/does/not/exist")

	if result.Disposition != DispositionConfirmed {
		t.Errorf("expected DispositionConfirmed, got %v", result.Disposition)
	}

	if !strings.Contains(result.Reason, "file unreadable") {
		t.Errorf("expected reason to contain 'file unreadable', got %q", result.Reason)
	}
}

func TestLLMFindingVerifier_ParseError(t *testing.T) {
	// AC 7: Returns DispositionConfirmed with reason containing 'parse error' when LLM response first word is not a known disposition
	ctx := context.Background()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Mock invoker returns an unknown disposition
	invoker := &mockInvoker{
		result: &provider.Result{
			Output: "unknown_disposition some explanation",
		},
	}

	verifier := NewLLMFindingVerifier(invoker)

	finding := Finding{
		Facet:       "test",
		Severity:    SeverityError,
		File:        "test.go",
		Line:        1,
		Description: "test finding",
		Cycle:       1,
	}

	result, _ := verifier.Verify(ctx, finding, tmpDir)

	if result.Disposition != DispositionConfirmed {
		t.Errorf("expected DispositionConfirmed, got %v", result.Disposition)
	}

	if !strings.Contains(result.Reason, "parse error") {
		t.Errorf("expected reason to contain 'parse error', got %q", result.Reason)
	}
}

func TestIsInDiffFiles_NoFalsePositiveSharedBasename(t *testing.T) {
	// Fix task: Verify that finding for 'cmd/bar/validate.go' returns false when only 'internal/foo/validate.go' is in diffFiles.
	// Previous implementation with basename loop incorrectly matched due to shared basename.
	diffFiles := map[string]bool{
		"internal/foo/validate.go": true,
	}

	// Finding for a different file with same basename should NOT match
	if isInDiffFiles("cmd/bar/validate.go", diffFiles) {
		t.Errorf("expected false for 'cmd/bar/validate.go' when only 'internal/foo/validate.go' is in diff, got true")
	}

	// Finding for the actual file in diff SHOULD match
	if !isInDiffFiles("internal/foo/validate.go", diffFiles) {
		t.Errorf("expected true for 'internal/foo/validate.go' when it is in diff, got false")
	}
}

func TestIsInDiffFiles_BareBasenameMatchesFullPathDiffEntry(t *testing.T) {
	// Fix task: When finding has a bare basename 'validate.go' (no path separator),
	// it should match a full-path entry in diffFiles like 'internal/next/specloop/stages/validate.go'.
	diffFiles := map[string]bool{
		"internal/next/specloop/stages/validate.go": true,
	}

	// Bare basename should match full-path entry
	if !isInDiffFiles("validate.go", diffFiles) {
		t.Errorf("expected true for bare basename 'validate.go' matching full-path entry in diff, got false")
	}

	// But should not match a different file with same basename
	diffFiles2 := map[string]bool{
		"internal/foo/validate.go": true,
	}
	if !isInDiffFiles("validate.go", diffFiles2) {
		t.Errorf("expected true for bare basename 'validate.go' matching different path entry in diff, got false")
	}

	// Bare basename should not match if no file with that basename exists
	diffFiles3 := map[string]bool{
		"internal/other/main.go": true,
	}
	if isInDiffFiles("validate.go", diffFiles3) {
		t.Errorf("expected false for bare basename 'validate.go' when no matching basename in diff, got true")
	}
}
