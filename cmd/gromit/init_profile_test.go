package main

import (
    "os"
    "path/filepath"
    "testing"
)

func TestDetectProfilePriority(t *testing.T) {
    cases := []struct {
        name  string
        files []string
        want  string
    }{
        {name: "go takes precedence", files: []string{"go.mod", "package.json"}, want: "go"},
        {name: "node before python", files: []string{"package.json", "pyproject.toml"}, want: "node"},
        {name: "python when only python signal", files: []string{"pyproject.toml"}, want: "python"},
        {name: "custom when nothing", files: nil, want: "custom"},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            dir := t.TempDir()
            for _, file := range tc.files {
                path := filepath.Join(dir, file)
                if err := os.WriteFile(path, []byte(""), 0644); err != nil {
                    t.Fatalf("write signal file %s: %v", file, err)
                }
            }

            if got := detectProfile(dir); got != tc.want {
                t.Fatalf("detectProfile() = %q, want %q", got, tc.want)
            }
        })
    }
}
