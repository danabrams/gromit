package testpkg

import (
    "io/fs"
    "os"
    "path/filepath"
    "sort"
    "strings"
)

// FindTaggedPackages walks root and returns package directories containing test
// files whose first few lines include //go:build or // +build expressions
// referencing tag. The returned paths are relative to root, prefixed with "./"
// except for the root itself which is returned as ".".
func FindTaggedPackages(root, tag string) ([]string, error) {
    absRoot, err := filepath.Abs(root)
    if err != nil {
        return nil, err
    }

    seen := map[string]struct{}{}
    err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            if path != absRoot && shouldSkipDir(d.Name()) {
                return filepath.SkipDir
            }
            return nil
        }

        if !strings.HasSuffix(d.Name(), "_test.go") {
            return nil
        }

        content, err := os.ReadFile(path)
        if err != nil {
            return err
        }

        if !hasBuildTag(content, tag) {
            return nil
        }

        dir := filepath.Dir(path)
        rel, err := filepath.Rel(absRoot, dir)
        if err != nil {
            return err
        }
        seen[normalizeRel(rel)] = struct{}{}
        return nil
    })
    if err != nil {
        return nil, err
    }

    pkgs := make([]string, 0, len(seen))
    for pkg := range seen {
        pkgs = append(pkgs, pkg)
    }
    sort.Strings(pkgs)
    return pkgs, nil
}

func hasBuildTag(content []byte, tag string) bool {
    lines := strings.Split(string(content), "\n")
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        switch {
        case strings.HasPrefix(trimmed, "//go:build"):
            expr := extractExpression(trimmed, "//go:build")
            if matchesTag(expr, tag) {
                return true
            }
        case strings.HasPrefix(trimmed, "// +build"):
            expr := extractExpression(trimmed, "// +build")
            if matchesTag(expr, tag) {
                return true
            }
        }
    }
    return false
}

func extractExpression(line, prefix string) string {
    expr := strings.TrimSpace(strings.TrimPrefix(line, prefix))
    if idx := strings.Index(expr, "//"); idx >= 0 {
        expr = expr[:idx]
    }
    return strings.TrimSpace(expr)
}

func matchesTag(expr, tag string) bool {
    if expr == "" {
        return false
    }
    fields := strings.FieldsFunc(expr, func(r rune) bool {
        switch r {
        case ' ', '\t', '\n', '\r', '(', ')', '!', '&', '|':
            return true
        default:
            return false
        }
    })
    for _, field := range fields {
        if field == tag {
            return true
        }
    }
    return false
}

func normalizeRel(rel string) string {
    rel = filepath.ToSlash(rel)
    if rel == "." || rel == "" {
        return "."
    }
    rel = strings.TrimPrefix(rel, "./")
    return "./" + rel
}

func shouldSkipDir(name string) bool {
    switch name {
    case ".git", ".gromit", ".beads", "vendor", "node_modules", "testdata":
        return true
    }
    if strings.HasPrefix(name, ".") {
        return true
    }
    return false
}
