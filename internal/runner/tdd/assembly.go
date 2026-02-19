package tdd

import "strings"

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
	testPaths, implPaths := ClassifyTouchedFiles(state.TouchedFiles)

	testFiles := make(map[string]string, len(testPaths))
	for _, p := range testPaths {
		content, err := readFile(p)
		if err != nil {
			return nil, err
		}
		testFiles[p] = content
	}

	implFiles := make(map[string]string, len(implPaths))
	for _, p := range implPaths {
		content, err := readFile(p)
		if err != nil {
			return nil, err
		}
		implFiles[p] = content
	}

	var specExcerpt string
	if len(state.Remaining) > 0 {
		specExcerpt = state.Remaining[0]
	}

	return &RedHandoff{
		SpecExcerpt: specExcerpt,
		TestFiles:   testFiles,
		ImplFiles:   implFiles,
	}, nil
}

// AssembleGreenHandoff builds context for the green (implementation) phase.
// It reads the test file just written and current impl files, capturing the
// test failure output for context.
func AssembleGreenHandoff(testOutput string, readFile ReadFileFn, touchedFiles []string) (*GreenHandoff, error) {
	testPaths, implPaths := ClassifyTouchedFiles(touchedFiles)

	// Read test files and concatenate as the failing test content
	var failingTest string
	for _, p := range testPaths {
		content, err := readFile(p)
		if err != nil {
			return nil, err
		}
		if failingTest != "" {
			failingTest += "\n"
		}
		failingTest += content
	}

	implFiles := make(map[string]string, len(implPaths))
	for _, p := range implPaths {
		content, err := readFile(p)
		if err != nil {
			return nil, err
		}
		implFiles[p] = content
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

	testFiles := make(map[string]string, len(testPaths))
	for _, p := range testPaths {
		content, err := readFile(p)
		if err != nil {
			return nil, err
		}
		testFiles[p] = content
	}

	implFiles := make(map[string]string, len(implPaths))
	for _, p := range implPaths {
		content, err := readFile(p)
		if err != nil {
			return nil, err
		}
		implFiles[p] = content
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

	return next
}
