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
	symbolTypeRE   = regexp.MustCompile(`^type\s+([A-Z]\w*(?:\[[^\]]+\])?)\b`)
	symbolMethodRE = regexp.MustCompile(`^func\s+\([^)]*\)\s+([A-Z]\w*)\s*\(`)
	structDeclRE   = regexp.MustCompile(`^type\s+\w+\s+struct\b`)
	structFieldRE  = regexp.MustCompile(`^([A-Z]\w*)\b`)
)

// ExtractSymbolsFromDiff parses diff output and returns exported symbols added in the patch.
func ExtractSymbolsFromDiff(diff string) []Symbol {
	scanner := bufio.NewScanner(strings.NewReader(diff))
	var symbols []Symbol
	var currentFile string
	var newLine int
	var structStack []int
	skipSymbolFile := false
	fileSymbolStart := 0

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
			structStack = nil
			skipSymbolFile = strings.HasSuffix(currentFile, "_test.go")
			fileSymbolStart = len(symbols)
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
			trimmed := strings.TrimSpace(line[1:])
			if strings.Contains(trimmed, "wiring:deferred") {
				skipSymbolFile = true
				symbols = symbols[:fileSymbolStart]
				continue
			}
			started := startStructContext(trimmed, &structStack)
			if !started {
				updateStructContext(trimmed, &structStack)
			}
		case '+':
			newLine++
			if currentFile == "" {
				continue
			}
			trimmed := strings.TrimSpace(line[1:])
			if strings.Contains(trimmed, "wiring:deferred") {
				skipSymbolFile = true
				symbols = symbols[:fileSymbolStart]
				continue
			}
			if skipSymbolFile {
				continue
			}
			started := startStructContext(trimmed, &structStack)
			if name := parseSymbolName(trimmed, structStack); name != "" {
				symbols = append(symbols, Symbol{Name: name, File: currentFile, Line: newLine})
			}
			if !started {
				updateStructContext(trimmed, &structStack)
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

func parseSymbolName(line string, structStack []int) string {
	if matches := symbolTypeRE.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}
	if matches := symbolMethodRE.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}
	if len(structStack) > 0 && structFieldRE.MatchString(line) {
		if matches := structFieldRE.FindStringSubmatch(line); len(matches) > 1 {
			return matches[1]
		}
	}
	if matches := symbolFuncRE.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func startStructContext(line string, stack *[]int) bool {
	if line == "" {
		return false
	}
	if structDeclRE.MatchString(line) {
		delta := strings.Count(line, "{") - strings.Count(line, "}")
		*stack = append(*stack, delta)
		return true
	}
	return false
}

func updateStructContext(line string, stack *[]int) {
	if len(line) == 0 || len(*stack) == 0 {
		return
	}
	delta := strings.Count(line, "{") - strings.Count(line, "}")
	if delta == 0 {
		return
	}
	top := len(*stack) - 1
	(*stack)[top] += delta
	for len(*stack) > 0 && (*stack)[len(*stack)-1] <= 0 {
		*stack = (*stack)[:len(*stack)-1]
	}
}
