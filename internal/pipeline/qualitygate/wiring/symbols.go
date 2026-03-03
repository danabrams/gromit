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
	hunkHeaderRE         = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
	symbolFuncRE         = regexp.MustCompile(`^func\s+([A-Z]\w*)\s*\(`)
	symbolTypeRE         = regexp.MustCompile(`^type\s+([A-Z]\w*(?:\[[^\]]+\])?)\b`)
	symbolMethodRE       = regexp.MustCompile(`^func\s+\([^)]*\)\s+([A-Z]\w*)\s*\(`)
	structDeclRE         = regexp.MustCompile(`^type\s+\w+\s+struct\b`)
	structFieldRE        = regexp.MustCompile(`^([A-Z]\w*)\b`)
	interfaceDeclRE      = regexp.MustCompile(`\binterface\b`)
	constDeclRE          = regexp.MustCompile(`^const\s+([A-Z]\w*)\b`)
	varDeclRE            = regexp.MustCompile(`^var\s+([A-Z]\w*)\b`)
	constBlockStartRE    = regexp.MustCompile(`^const\s*\(`)
	varBlockStartRE      = regexp.MustCompile(`^var\s*\(`)
	constVarBlockFieldRE = regexp.MustCompile(`^([A-Z]\w*)\b`)
	typeBlockStartRE     = regexp.MustCompile(`^type\s*\(`)
	typeBlockEntryRE     = regexp.MustCompile(`^([A-Z]\w*(?:\[[^\]]+\])?)\b`)
)

// ExtractSymbolsFromDiff parses diff output and returns exported symbols added in the patch.
func ExtractSymbolsFromDiff(diff string) []Symbol {
	scanner := bufio.NewScanner(strings.NewReader(diff))
	var symbols []Symbol
	var currentFile string
	var newLine int
	var structStack []int
	var interfaceStack []int
	var constBlockActive bool
	var varBlockActive bool
	var typeBlockActive bool
	skipSymbolFile := false
	deferredNext := false

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
			interfaceStack = nil
			typeBlockActive = false
			skipSymbolFile = strings.HasSuffix(currentFile, "_test.go")
			deferredNext = false
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
			structStarted := startStructContext(trimmed, &structStack)
			interfaceStarted := startInterfaceContext(trimmed, &interfaceStack)
			typeStarted := startTypeContext(trimmed, &typeBlockActive)
			constStarted := startConstContext(trimmed, &constBlockActive)
			varStarted := startVarContext(trimmed, &varBlockActive)
			finalizeContexts(trimmed, structStarted, interfaceStarted, &structStack, &interfaceStack, typeStarted, constStarted, varStarted, &typeBlockActive, &constBlockActive, &varBlockActive)
		case '+':
			newLine++
			if currentFile == "" {
				continue
			}
			trimmed := strings.TrimSpace(line[1:])
			if skipSymbolFile {
				continue
			}
			structStarted := startStructContext(trimmed, &structStack)
			interfaceStarted := startInterfaceContext(trimmed, &interfaceStack)
			typeStarted := startTypeContext(trimmed, &typeBlockActive)
			constStarted := startConstContext(trimmed, &constBlockActive)
			varStarted := startVarContext(trimmed, &varBlockActive)

			if deferredNext {
				deferredNext = false
				finalizeContexts(trimmed, structStarted, interfaceStarted, &structStack, &interfaceStack, typeStarted, constStarted, varStarted, &typeBlockActive, &constBlockActive, &varBlockActive)
				continue
			}
			if strings.HasPrefix(trimmed, "//") && strings.Contains(trimmed, "wiring:deferred") {
				deferredNext = true
				finalizeContexts(trimmed, structStarted, interfaceStarted, &structStack, &interfaceStack, typeStarted, constStarted, varStarted, &typeBlockActive, &constBlockActive, &varBlockActive)
				continue
			}

			inInterface := len(interfaceStack) > 0 && !interfaceStarted
			if inInterface {
				finalizeContexts(trimmed, structStarted, interfaceStarted, &structStack, &interfaceStack, typeStarted, constStarted, varStarted, &typeBlockActive, &constBlockActive, &varBlockActive)
				continue
			}

			if name := parseSymbolName(trimmed, structStack, constBlockActive, varBlockActive, typeBlockActive); name != "" {
				symbols = append(symbols, Symbol{Name: name, File: currentFile, Line: newLine})
			}
			finalizeContexts(trimmed, structStarted, interfaceStarted, &structStack, &interfaceStack, typeStarted, constStarted, varStarted, &typeBlockActive, &constBlockActive, &varBlockActive)
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

func parseSymbolName(line string, structStack []int, inConstBlock, inVarBlock, inTypeBlock bool) string {
	if inTypeBlock {
		if matches := typeBlockEntryRE.FindStringSubmatch(line); len(matches) > 1 {
			return matches[1]
		}
	}
	if inConstBlock {
		if name := parseConstVarBlockSymbol(line); name != "" {
			return name
		}
	}
	if inVarBlock {
		if name := parseConstVarBlockSymbol(line); name != "" {
			return name
		}
	}
	if matches := constDeclRE.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}
	if matches := varDeclRE.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}
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

func parseConstVarBlockSymbol(line string) string {
	if matches := constVarBlockFieldRE.FindStringSubmatch(line); len(matches) > 1 {
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

func startInterfaceContext(line string, stack *[]int) bool {
	if line == "" {
		return false
	}
	if interfaceDeclRE.MatchString(line) && strings.Contains(line, "{") {
		delta := strings.Count(line, "{") - strings.Count(line, "}")
		*stack = append(*stack, delta)
		return true
	}
	return false
}

func updateInterfaceContext(line string, stack *[]int) {
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

func startConstContext(line string, active *bool) bool {
	if line == "" {
		return false
	}
	if constBlockStartRE.MatchString(line) {
		*active = true
		return true
	}
	return false
}

func updateConstContext(line string, active *bool) {
	if !*active {
		return
	}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, ")") {
		return
	}
	rest := strings.TrimSpace(trimmed[1:])
	if rest == "" || strings.HasPrefix(rest, "//") || strings.HasPrefix(rest, "/*") {
		*active = false
	}
}

func startVarContext(line string, active *bool) bool {
	if line == "" {
		return false
	}
	if varBlockStartRE.MatchString(line) {
		*active = true
		return true
	}
	return false
}

func updateVarContext(line string, active *bool) {
	if !*active {
		return
	}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, ")") {
		return
	}
	rest := strings.TrimSpace(trimmed[1:])
	if rest == "" || strings.HasPrefix(rest, "//") || strings.HasPrefix(rest, "/*") {
		*active = false
	}
}

func startTypeContext(line string, active *bool) bool {
	if line == "" {
		return false
	}
	if typeBlockStartRE.MatchString(line) {
		*active = true
		return true
	}
	return false
}

func updateTypeContext(line string, active *bool) {
	if !*active {
		return
	}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, ")") {
		return
	}
	rest := strings.TrimSpace(trimmed[1:])
	if rest == "" || strings.HasPrefix(rest, "//") || strings.HasPrefix(rest, "/*") {
		*active = false
	}
}

func finalizeContexts(line string, structStarted, interfaceStarted bool, structStack *[]int, interfaceStack *[]int, typeStarted, constStarted, varStarted bool, typeActive, constActive, varActive *bool) {
	if !structStarted {
		updateStructContext(line, structStack)
	}
	if !interfaceStarted {
		updateInterfaceContext(line, interfaceStack)
	}
	if !typeStarted {
		updateTypeContext(line, typeActive)
	}
	if !constStarted {
		updateConstContext(line, constActive)
	}
	if !varStarted {
		updateVarContext(line, varActive)
	}
}
