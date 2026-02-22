package tdd

import (
	"fmt"
	"strings"
)

// ReadFileFn reads a file and returns its content.
type ReadFileFn func(path string) (string, error)

// GetDiffFn returns the current git diff.
type GetDiffFn func() (string, error)

// RunTestsFn runs tests and returns the output.
type RunTestsFn func() (string, error)

// ClassifyTouchedFiles separates paths into test files (_test.go) and implementation files (.go).
func ClassifyTouchedFiles(paths []string) (testFiles, implFiles []string) {
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			testFiles = append(testFiles, p)
		} else {
			implFiles = append(implFiles, p)
		}
	}
	return testFiles, implFiles
}

// AssembleRedHandoff builds context for the red (test-writing) phase.
// It reads current test/impl files from state.TouchedFiles and uses
// state.Remaining[0] as the spec excerpt for the next requirement.
func AssembleRedHandoff(state CycleState, readFile ReadFileFn, getDiff GetDiffFn) (*RedHandoff, error) {
	// Diff collection is handled by callers today; keep dependency explicit for testability.
	_ = getDiff

	testPaths, implPaths := ClassifyTouchedFiles(state.TouchedFiles)
	testFiles, err := readFiles(readFile, testPaths)
	if err != nil {
		return nil, err
	}
	implFiles, err := readFiles(readFile, implPaths)
	if err != nil {
		return nil, err
	}

	var specExcerpt string
	if len(state.Remaining) > 0 {
		specExcerpt = state.Remaining[0]
	}

	return &RedHandoff{
		SpecExcerpt:  specExcerpt,
		TestFiles:    testFiles,
		ImplFiles:    implFiles,
		APISurface:   extractAPISurface(implFiles),
		CycleSummary: buildCycleSummary(state),
	}, nil
}

func buildCycleSummary(state CycleState) string {
	if len(state.CoveredSoFar) == 0 && len(state.Remaining) == 0 {
		return ""
	}

	var sb strings.Builder
	if len(state.CoveredSoFar) > 0 {
		sb.WriteString("Completed:\n")
		for i, req := range state.CoveredSoFar {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, req))
		}
	}
	if len(state.Remaining) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Remaining:\n")
		for i, req := range state.Remaining {
			sb.WriteString(fmt.Sprintf("%d. %s\n", len(state.CoveredSoFar)+i+1, req))
		}
	}
	testFiles, implFiles := ClassifyTouchedFiles(state.TouchedFiles)
	if len(testFiles) > 0 || len(implFiles) > 0 {
		sb.WriteString("\nTouched files:\n")
		for _, f := range testFiles {
			sb.WriteString(fmt.Sprintf("  [test] %s\n", f))
		}
		for _, f := range implFiles {
			sb.WriteString(fmt.Sprintf("  [impl] %s\n", f))
		}
	}
	return sb.String()
}

// AssembleGreenHandoff builds context for the green (implementation) phase.
// It reads the test file just written and current impl files, capturing the
// test failure output for context.
func AssembleGreenHandoff(testOutput string, readFile ReadFileFn, touchedFiles []string) (*GreenHandoff, error) {
	testPaths, implPaths := ClassifyTouchedFiles(touchedFiles)

	failingTest, err := readAndJoinFiles(readFile, testPaths)
	if err != nil {
		return nil, err
	}
	implFiles, err := readFiles(readFile, implPaths)
	if err != nil {
		return nil, err
	}

	return &GreenHandoff{
		FailingTest:       failingTest,
		TestFailureOutput: testOutput,
		ImplFiles:         implFiles,
	}, nil
}

// AssembleRefactorHandoff builds context for the refactor phase.
// It reads all touched test and impl files for behavior-preserving cleanup.
func AssembleRefactorHandoff(readFile ReadFileFn, touchedFiles []string) (*RefactorHandoff, error) {
	testPaths, implPaths := ClassifyTouchedFiles(touchedFiles)
	testFiles, err := readFiles(readFile, testPaths)
	if err != nil {
		return nil, err
	}
	implFiles, err := readFiles(readFile, implPaths)
	if err != nil {
		return nil, err
	}

	return &RefactorHandoff{
		ImplFiles: implFiles,
		TestFiles: testFiles,
	}, nil
}

// AssembleCycleState creates the next cycle state from the previous one.
// It increments the cycle number and moves the first remaining requirement
// to covered.
func AssembleCycleState(prevState CycleState, redOutput string) CycleState {
	_ = redOutput

	next := CycleState{
		CycleNumber:  prevState.CycleNumber + 1,
		MaxCycles:    prevState.MaxCycles,
		TouchedFiles: prevState.TouchedFiles,
		Done:         prevState.Done,
	}

	// Copy covered and move first remaining to covered
	next.CoveredSoFar = make([]string, len(prevState.CoveredSoFar))
	copy(next.CoveredSoFar, prevState.CoveredSoFar)

	if len(prevState.Remaining) > 0 {
		next.CoveredSoFar = append(next.CoveredSoFar, prevState.Remaining[0])
		next.Remaining = make([]string, len(prevState.Remaining)-1)
		copy(next.Remaining, prevState.Remaining[1:])
	} else {
		next.Remaining = []string{}
	}

	if len(next.Remaining) == 0 {
		next.Done = true
	}

	return next
}

func readFiles(readFile ReadFileFn, paths []string) (map[string]string, error) {
	files := make(map[string]string, len(paths))
	for _, p := range paths {
		content, err := readFile(p)
		if err != nil {
			return nil, err
		}
		files[p] = content
	}
	return files, nil
}

func extractAPISurface(implFiles map[string]string) string {
	var lines []string
	for _, content := range implFiles {
		lines = append(lines, extractFileAPISurface(content)...)
	}
	return strings.Join(lines, "\n")
}

func extractFileAPISurface(content string) []string {
	var lines []string
	inInterface := false
	inConstVarBlock := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Track if we're inside an interface block
		if strings.Contains(line, "interface {") {
			inInterface = true
		}
		if inInterface && trimmed == "}" {
			inInterface = false
		}

		// Track if we're inside a const ( or var ( block
		if strings.HasPrefix(line, "const (") || strings.HasPrefix(line, "var (") {
			inConstVarBlock = true
			lines = append(lines, trimmed)
			continue
		}
		if inConstVarBlock {
			if trimmed == ")" {
				inConstVarBlock = false
			} else if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
				lines = append(lines, trimmed)
			}
			continue
		}

		// Capture top-level declarations
		if strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") {
			decl := line
			if idx := strings.Index(decl, "{"); idx >= 0 {
				decl = strings.TrimSpace(decl[:idx])
			}
			lines = append(lines, decl)
		} else if strings.HasPrefix(line, "const ") || strings.HasPrefix(line, "var ") {
			// Capture single-line const/var declarations
			lines = append(lines, trimmed)
		} else if inInterface && isMethodSignature(trimmed) {
			// Capture interface method signatures
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func isMethodSignature(line string) bool {
	// Skip empty lines and comments
	if line == "" || strings.HasPrefix(line, "//") {
		return false
	}
	// Method signatures have the pattern: name(params)returntype
	// Must contain parentheses
	return strings.Contains(line, "(") && strings.Contains(line, ")")
}

func readAndJoinFiles(readFile ReadFileFn, paths []string) (string, error) {
	contents, err := readFiles(readFile, paths)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, len(paths))
	for _, p := range paths {
		parts = append(parts, contents[p])
	}
	return strings.Join(parts, "\n"), nil
}
