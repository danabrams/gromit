package testpkg

import (
    "os"
    "path/filepath"
    "reflect"
    "sort"
    "testing"
)

func TestFindTaggedPackages(t *testing.T) {
    root := t.TempDir()

    createFile(t, root, "pkg_live/tagged_test.go", "//go:build e2e_live\npackage live_test\n")
    createFile(t, root, "pkg_live/nested/tagged_sub_test.go", "//go:build e2e_live\npackage live_test\n")
    createFile(t, root, "pkg_other/untagged_test.go", "package other_test\n")
    createFile(t, root, "pkg_acceptance/acceptance_test.go", "//go:build acceptance\npackage acc_test\n")

    pkgs, err := FindTaggedPackages(root, "e2e_live")
    if err != nil {
        t.Fatalf("FindTaggedPackages returned error: %v", err)
    }

    sort.Strings(pkgs)
    want := []string{"./pkg_live", "./pkg_live/nested"}
    if !reflect.DeepEqual(pkgs, want) {
        t.Fatalf("unexpected packages\nwant: %v\ngot:  %v", want, pkgs)
    }
}

func createFile(t *testing.T, root, relPath, content string) {
    t.Helper()
    fullPath := filepath.Join(root, relPath)
    if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
        t.Fatalf("mkdir failed: %v", err)
    }
    if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
        t.Fatalf("write failed: %v", err)
    }
}
