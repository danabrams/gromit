package learnings

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestGromitMetadataNoiseAbsent(t *testing.T) {
    root := findRepoRoot(t)
    check := func(t *testing.T, path string, snippets []string) {
        t.Helper()
        data, err := os.ReadFile(path)
        if err != nil {
            t.Fatalf("failed to read %s: %v", path, err)
        }
        for _, snippet := range snippets {
            if strings.Contains(string(data), snippet) {
                t.Fatalf("unexpected gromit metadata noise (%s) still present in %s", snippet, path)
            }
        }
    }

    check(t, filepath.Join(root, ".gromit", "RULES.md"), []string{
        "Never ignore loader errors",
        "Do not discard errors from renderer/template/rules loaders",
        "During orchestrator migrations, cross-cutting concerns (state persistence, cost/token metrics, status updates) must be implemented in one shared path",
        "Usage accounting must use explicit before/after snapshots",
    })

    check(t, filepath.Join(root, ".gromit", "LEARNINGS.md"), []string{
        "gromit-sjg0",
        "gromit-qs2ks",
    })
}

func findRepoRoot(t *testing.T) string {
    t.Helper()
    dir, err := os.Getwd()
    if err != nil {
        t.Fatalf("failed to determine working directory: %v", err)
    }
    for {
        if info, err := os.Stat(filepath.Join(dir, ".gromit")); err == nil && info.IsDir() {
            return dir
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            t.Fatalf("could not locate repository root containing .gromit")
        }
        dir = parent
    }
}
