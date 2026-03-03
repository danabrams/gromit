package wiring

import (
    "bufio"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
)

// Symbol represents a newly-added exported Go symbol in a diff patch.
type Symbol struct {
    Name string
    File string
    Line int
}

var (
    hunkHeaderRE   = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
    symbolFuncRE   = regexp.MustCompile(`^func\s+([A-Z]\w*)\s*\(`)
)

// ExtractSymbolsFromDiff parses diff output and returns exported symbols added in the patch.
func ExtractSymbolsFromDiff(diff string) []Symbol {
    scanner := bufio.NewScanner(strings.NewReader(diff))
    var symbols []Symbol
    var currentFile string
    var newLine int

    for scanner.Scan() {
        line := scanner.Text()

        if strings.HasPrefix(line, "diff --git") {
            currentFile = ""
            continue
        }
        if strings.HasPrefix(line, "+++ ") {
            raw := strings.TrimPrefix(line, "+++ ")
            if raw == "/dev/null" {
                currentFile = ""
                continue
            }
            if strings.HasPrefix(raw, "b/") {
                raw = strings.TrimPrefix(raw, "b/")
            }
            currentFile = filepath.ToSlash(raw)
            continue
        }
        if strings.HasPrefix(line, "@@ ") {
            matches := hunkHeaderRE.FindStringSubmatch(line)
            if len(matches) > 1 {
                start, _ := strconv.Atoi(matches[1])
                newLine = start - 1
            }
            continue
        }

        if line == "" {
            continue
        }

        switch line[0] {
        case ' ':
            newLine++
        case '+':
            newLine++
            if currentFile == "" {
                continue
            }
            trimmed := strings.TrimSpace(line[1:])
            if name := parseFuncName(trimmed); name != "" {
                symbols = append(symbols, Symbol{Name: name, File: currentFile, Line: newLine})
            }
        case '-':
            // removal lines do not affect new file line numbers
        default:
            // ignore other prefixes
        }
    }

    return symbols
}

func parseFuncName(line string) string {
    if matches := symbolFuncRE.FindStringSubmatch(line); len(matches) > 1 {
        return matches[1]
    }
    return ""
}
