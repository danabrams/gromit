package runner

import (
    "bytes"
    "go/format"
    "os"
    "path/filepath"
    "testing"
)

func TestGofmtCompliance(t *testing.T) {
    files := []string{
        "spc_auto_triage.go",
        "specmerge/pr_summary.go",
    }

    for _, path := range files {
        path := path
        t.Run(filepath.Base(path), func(t *testing.T) {
            t.Helper()

            src, err := os.ReadFile(path)
            if err != nil {
                t.Fatalf("reading %s: %v", path, err)
            }

            formatted, err := format.Source(src)
            if err != nil {
                t.Fatalf("formatting %s: %v", path, err)
            }

            if !bytes.Equal(src, formatted) {
                t.Fatalf("%s is not gofmt-compliant", path)
            }
        })
    }
}
