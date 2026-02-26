package repohygiene

import (
    "path/filepath"
    "runtime"
    "strings"
    "testing"
)

func TestRepoRootHasNoStrayScratchNotes(t *testing.T) {
    _, thisFile, _, ok := runtime.Caller(0)
    if !ok {
        t.Fatal("runtime.Caller failed")
    }
    repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

    strayFiles := []string{"debug.md", "devug.md", "fixed.md", "fixes.md", "testfix.md"}
    found, err := findStrayScratchFiles(repoRoot, strayFiles)
    if err != nil {
        t.Fatalf("check stray scratch files: %v", err)
    }
    if len(found) > 0 {
        t.Fatalf("unexpected stray scratch notes at repo root: %s", strings.Join(found, ", "))
    }
}
