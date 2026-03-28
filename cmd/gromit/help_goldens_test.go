package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadGolden(t *testing.T, path string) string {
	t.Helper()
	var data []byte
	var err error
	for _, candidate := range []string{
		path,
		filepath.Join("..", path),
		filepath.Join("..", "..", path),
	} {
		data, err = os.ReadFile(candidate)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			t.Fatalf("reading golden file %q (tried %q): %v", path, candidate, err)
		}
	}
	if err != nil {
		t.Fatalf("reading golden file %q: %v", path, err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func assertHelpMatchesGolden(t *testing.T, path string, actual string) {
	t.Helper()
	expected := loadGolden(t, path)
	if actual != expected {
		t.Fatalf("command help differs from golden %q\nexpected:\n%s\n\ngot:\n%s", path, expected, actual)
	}
}

func TestRootHelpGoldenIncludesPRS(t *testing.T) {
	t.Parallel()

	content := loadGolden(t, "cmd/gromit/testdata/golden/root.help.txt")
	if !strings.Contains(content, "  prs           ") {
		t.Fatalf("root help golden output missing prs entry:\n%s", content)
	}
}

func TestRootHelpMentionsDebugJobs(t *testing.T) {
	buf := new(bytes.Buffer)
	oldOut := rootCmd.OutOrStdout()
	rootCmd.SetOut(buf)
	defer rootCmd.SetOut(oldOut)

	if err := rootCmd.Help(); err != nil {
		t.Fatalf("failed to render root help: %v", err)
	}

	text := strings.ToLower(buf.String())
	for _, job := range []string{"diagnose", "fix", "learn"} {
		if !strings.Contains(text, job) {
			t.Fatalf("root help missing mention of %s job: %s", job, text)
		}
	}
}
