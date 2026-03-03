package gromit

import (
	"os"
	"strings"
	"testing"
)

func TestVetBuildResultsRecorded(t *testing.T) {
	const (
		resultsFile = "vet-build-results.txt"
	)

	data, err := os.ReadFile(resultsFile)
	if err != nil {
		t.Fatalf("read %s: %v", resultsFile, err)
	}

	content := string(data)

	for _, command := range []string{
		"go vet ./...",
		"go build ./...",
	} {
		if !strings.Contains(content, command) {
			t.Errorf("%s missing %q entry", resultsFile, command)
		}
	}
}
