package main

import (
	"strings"
	"testing"
)

func TestCmdGromitTestFilesGofmt(t *testing.T) {
	files := cmdGromitTestFiles(t)
	if len(files) == 0 {
		t.Fatalf("no cmd/gromit test files found")
	}

	if nonCompliant := gofmtNonCompliantFiles(t, files); len(nonCompliant) > 0 {
		t.Fatalf("gofmt -l reported non-compliant files:\n%s", strings.Join(nonCompliant, "\n"))
	}
}
