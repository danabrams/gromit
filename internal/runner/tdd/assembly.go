package tdd

import "strings"

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
