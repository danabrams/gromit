package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// VerifierDisposition represents the outcome of verifying a code review finding.
type VerifierDisposition string

const (
	DispositionConfirmed  VerifierDisposition = "confirmed"
	DispositionDowngraded VerifierDisposition = "downgraded"
	DispositionFixed      VerifierDisposition = "fixed"
)

// VerifierResult contains the result of verifying a single finding.
type VerifierResult struct {
	Finding     Finding
	Disposition VerifierDisposition
	Reason      string
	FileExcerpt string
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields satisfies the codebase convention. VerifierResult has no slice/map fields.
func (vr *VerifierResult) NormalizeNilFields() {}

// VerifierAuditEntry is the flat structure for JSON serialization to the audit log.
// It contains the flattened fields from a VerifierResult with snake_case JSON tags.
type VerifierAuditEntry struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
	FileExcerpt string `json:"file_excerpt"`
}

// NormalizeNilFields satisfies the codebase convention. VerifierAuditEntry has no slice/map fields.
func (vae *VerifierAuditEntry) NormalizeNilFields() {}

// FindingVerifier defines the interface for verifying code review findings.
type FindingVerifier interface {
	Verify(ctx context.Context, f Finding, workDir string) (VerifierResult, error)
}

// FilesInDiff extracts the set of modified file paths from a unified diff string.
// A line beginning with "+++ b/" introduces a modified file; the prefix is stripped
// and the path is normalized to a relative path.
// Returns an empty map when diffText is empty or contains no "+++ b/" lines.
func FilesInDiff(diffText string) map[string]bool {
	filesInDiff := make(map[string]bool)

	if diffText == "" {
		return filesInDiff
	}

	lines := strings.Split(diffText, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ b/") {
			// Extract the file path after "b/"
			filePath := strings.TrimPrefix(line, "+++ b/")
			if filePath != "" {
				filesInDiff[filePath] = true
			}
		}
	}

	return filesInDiff
}

// LLMFindingVerifier verifies code review findings using an LLM.
type LLMFindingVerifier struct {
	invoker llmadapter.Invoker
}

// NewLLMFindingVerifier creates a new LLMFindingVerifier.
func NewLLMFindingVerifier(invoker llmadapter.Invoker) *LLMFindingVerifier {
	return &LLMFindingVerifier{
		invoker: invoker,
	}
}

// Verify checks a code review finding using the LLM.
// It reads lines from max(0, line-5) to min(EOF, line+10) from the file,
// builds a verifier prompt, calls the invoker, and parses the first word
// of the response as the disposition.
//
// Returns DispositionConfirmed with 'file unreadable' reason on read error.
// Returns DispositionConfirmed with 'parse error' reason on unknown disposition.
func (v *LLMFindingVerifier) Verify(ctx context.Context, f Finding, workDir string) (VerifierResult, error) {
	filePath := filepath.Join(workDir, f.File)

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return VerifierResult{
			Finding:     f,
			Disposition: DispositionConfirmed,
			Reason:      "file unreadable — retaining finding",
		}, nil
	}

	// Split into lines
	lines := strings.Split(string(content), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Calculate line range: max(0, line-5) to min(EOF, line+10)
	startLine := f.Line - 6
	if startLine < 0 {
		startLine = 0
	}
	endLine := f.Line + 10
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Extract excerpt (convert 1-based to 0-based indexing)
	var excerpt string
	if startLine < len(lines) && endLine > 0 {
		excerpt = strings.Join(lines[startLine:endLine], "\n")
	}

	// Calculate display line numbers (1-indexed for display)
	startLineDisplay := f.Line - 5
	if startLineDisplay < 1 {
		startLineDisplay = 1
	}

	// Build prompt from spec template
	prompt := fmt.Sprintf(`You are verifying a code review finding against the actual current source code.

Finding: %s %s:%d — %s

Current file contents at that location (lines %d to %d):
%s

Is this finding still valid?
- confirmed: the described problem is present in this code
- downgraded: a real issue exists but it is less severe than "error"
- fixed: the code no longer has this problem

Return exactly one word (confirmed / downgraded / fixed) followed by a single sentence explaining why.`, f.Severity.String(), f.File, f.Line, f.Description, startLineDisplay, endLine, excerpt)

	// Call invoker
	result, err := v.invoker.Invoke(ctx, prompt)
	if err != nil {
		return VerifierResult{
			Finding:     f,
			Disposition: DispositionConfirmed,
			Reason:      "invoke error — retaining finding",
		}, nil
	}

	if result == nil {
		return VerifierResult{
			Finding:     f,
			Disposition: DispositionConfirmed,
			Reason:      "invoke error — retaining finding",
		}, nil
	}

	// Parse first word as disposition
	output := strings.TrimSpace(result.Output)
	words := strings.Fields(output)
	if len(words) == 0 {
		return VerifierResult{
			Finding:     f,
			Disposition: DispositionConfirmed,
			Reason:      "parse error — retaining finding",
		}, nil
	}

	firstWord := strings.ToLower(words[0])
	var disposition VerifierDisposition
	switch firstWord {
	case "confirmed":
		disposition = DispositionConfirmed
	case "downgraded":
		disposition = DispositionDowngraded
	case "fixed":
		disposition = DispositionFixed
	default:
		return VerifierResult{
			Finding:     f,
			Disposition: DispositionConfirmed,
			Reason:      "parse error — retaining finding",
		}, nil
	}

	return VerifierResult{
		Finding:     f,
		Disposition: disposition,
		Reason:      strings.TrimSpace(strings.Join(words[1:], " ")),
		FileExcerpt: excerpt,
	}, nil
}

// VerifyBlockingFindings verifies code review findings, filtering based on whether they
// are in the diff or not. In-diff findings are kept unchanged. Out-of-diff findings are
// verified using the provided verifier.
//
// Findings matching in diffFiles (by full path or basename) are considered in-diff.
// For out-of-diff findings:
//   - Confirmed: kept unchanged
//   - Downgraded: Severity set to SeverityWarning and kept
//   - Fixed: removed
//
// Results are returned in the same order as input findings.
// Returns (kept findings, all verification results for out-of-diff findings).
func VerifyBlockingFindings(ctx context.Context, findings []Finding, diffFiles map[string]bool, verifier FindingVerifier, workDir string) ([]Finding, []VerifierResult) {
	// Map to collect verification results by index
	resultMap := make(map[int]VerifierResult)
	resultsChan := make(chan struct {
		index  int
		result VerifierResult
	}, len(findings))
	var goroutineCount int

	// Spawn goroutines for out-of-diff findings
	for i, f := range findings {
		if !isInDiffFiles(f.File, diffFiles) {
			goroutineCount++
			go func(index int, finding Finding) {
				result, verifyErr := verifier.Verify(ctx, finding, workDir)
				if verifyErr != nil {
					// Fail-safe: treat verifier errors as DispositionConfirmed
					result = VerifierResult{
						Finding:     finding,
						Disposition: DispositionConfirmed,
						Reason:      "verification error — retaining finding",
					}
				}
				resultsChan <- struct {
					index  int
					result VerifierResult
				}{index, result}
			}(i, f)
		}
	}

	// Collect verification results
	for j := 0; j < goroutineCount; j++ {
		res := <-resultsChan
		resultMap[res.index] = res.result
	}

	// Build output in stable input order
	var kept []Finding
	var verifierResults []VerifierResult
	for i, f := range findings {
		if isInDiffFiles(f.File, diffFiles) {
			// In-diff finding: keep unchanged
			kept = append(kept, f)
		} else {
			// Out-of-diff finding: apply verification result
			result := resultMap[i]
			verifierResults = append(verifierResults, result)
			switch result.Disposition {
			case DispositionConfirmed:
				kept = append(kept, result.Finding)
			case DispositionDowngraded:
				f.Severity = SeverityWarning
				kept = append(kept, f)
			case DispositionFixed:
				// Don't add (remove from kept)
			default:
				kept = append(kept, f)
			}
		}
	}

	return kept, verifierResults
}

// isInDiffFiles checks if a file path matches a file in diffFiles.
// Matches on exact full path or basename.
// For bare basenames (no path separator), also checks if the basename matches
// any full-path entry in diffFiles.
func isInDiffFiles(file string, diffFiles map[string]bool) bool {
	// First check: exact full-path match
	if diffFiles[file] {
		return true
	}

	// Second check: basename match (handles both full paths and basenames)
	if diffFiles[filepath.Base(file)] {
		return true
	}

	// Third check: for bare basenames, check if any diffFiles key has matching basename
	// This handles the case where finding has "validate.go" and diffFiles has "internal/.../validate.go"
	// Only apply to bare basenames (no path separator) to avoid false positives
	if !strings.Contains(file, string(os.PathSeparator)) {
		for key := range diffFiles {
			if filepath.Base(key) == file {
				return true
			}
		}
	}

	return false
}
