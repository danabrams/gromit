package contract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type scenarioInfo struct {
	idx        int
	normalized string
}

type goTestKey struct {
	pkg      string
	testName string
}

// AugmentWithTestAssertions scans the worktree for scenario test files, matches
// them to scenarios, and injects go_test_pass assertions while preserving
// structural assertions. Content-based assertions are dropped when at least
// one matching go_test_pass exists.
func AugmentWithTestAssertions(sc *ScenarioContract, workDir string) error {
	if sc == nil || len(sc.Scenarios) == 0 {
		return nil
	}
	if workDir == "" {
		workDir = "."
	}
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return fmt.Errorf("resolve workdir %q: %w", workDir, err)
	}

	scenarioInfos := make([]scenarioInfo, 0, len(sc.Scenarios))
	for idx, scenario := range sc.Scenarios {
		scenarioInfos = append(scenarioInfos, scenarioInfo{
			idx:        idx,
			normalized: normalizeMatchingKey(scenario.Name),
		})
	}

	files, err := findScenarioTestFiles(absWorkDir)
	if err != nil {
		return fmt.Errorf("discover scenario tests: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	testsByScenario := make(map[int]map[goTestKey]GoTestPassAssertion)
	for _, file := range files {
		funcs, err := extractScenarioTestFunctions(file)
		if err != nil {
			return fmt.Errorf("parse scenario test %q: %w", file, err)
		}
		if len(funcs) == 0 {
			continue
		}
		pkgPath, err := packagePath(absWorkDir, file)
		if err != nil {
			return fmt.Errorf("compute package path for %q: %w", file, err)
		}
		for _, fn := range funcs {
			remainder := stripScenarioPrefix(fn)
			if remainder == "" {
				continue
			}
			normalized := normalizeMatchingKey(remainder)
			if normalized == "" {
				continue
			}
			matches := bestScenarioIndices(normalized, scenarioInfos)
			if len(matches) == 0 {
				continue
			}
			for _, idx := range matches {
				if testsByScenario[idx] == nil {
					testsByScenario[idx] = make(map[goTestKey]GoTestPassAssertion)
				}
				key := goTestKey{pkg: pkgPath, testName: fn}
				testsByScenario[idx][key] = GoTestPassAssertion{Pkg: pkgPath, TestName: fn}
			}
		}
	}

	for idx, scenario := range sc.Scenarios {
		goTests := testsByScenario[idx]
		filtered := filterAssertions(scenario.Assertions, len(goTests) > 0)
		if len(goTests) > 0 {
			filtered = append(filtered, sortedGoTestAssertions(goTests)...)
		}
		sc.Scenarios[idx].Assertions = filtered
	}

	return nil
}

func findScenarioTestFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.Contains(name, "_scenario_") && strings.HasSuffix(name, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func extractScenarioTestFunctions(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var tests []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name == nil {
			continue
		}
		if strings.HasPrefix(fn.Name.Name, "TestScenario") {
			tests = append(tests, fn.Name.Name)
		}
	}
	return tests, nil
}

func packagePath(workDir string, filePath string) (string, error) {
	dir := filepath.Dir(filePath)
	rel, err := filepath.Rel(workDir, dir)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return ".", nil
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%q outside workdir %q", dir, workDir)
	}
	return "./" + rel + "/...", nil
}

func normalizeMatchingKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r == '_' {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			continue
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func stripScenarioPrefix(name string) string {
	trimmed := strings.TrimPrefix(name, "TestScenario")
	trimmed = strings.TrimPrefix(trimmed, "_")
	return trimmed
}

func bestScenarioIndices(testKey string, scenarios []scenarioInfo) []int {
	if testKey == "" {
		return nil
	}
	bestScore := 0
	var matches []int
	for _, info := range scenarios {
		if info.normalized == "" {
			continue
		}
		score := matchingScore(info.normalized, testKey)
		if score == 0 {
			continue
		}
		if score > bestScore {
			bestScore = score
			matches = []int{info.idx}
		} else if score == bestScore {
			matches = append(matches, info.idx)
		}
	}
	if bestScore == 0 {
		return nil
	}
	// Require a unique best match; skip assignment if there are ties
	if len(matches) != 1 {
		return nil
	}
	return matches
}

func matchingScore(a, b string) int {
	if strings.Contains(a, b) {
		return len(b)
	}
	if strings.Contains(b, a) {
		return len(a)
	}
	return longestCommonSubstringLen(a, b)
}

func longestCommonSubstringLen(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	prev := make([]int, len(b)+1)
	best := 0
	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
				if curr[j] > best {
					best = curr[j]
				}
			}
		}
		prev = curr
	}
	return best
}

func filterAssertions(assertions []ContractAssertion, dropContent bool) []ContractAssertion {
	if !dropContent {
		return append([]ContractAssertion(nil), assertions...)
	}
	filtered := make([]ContractAssertion, 0, len(assertions))
	for _, assertion := range assertions {
		if assertion.FileContains != nil || assertion.FileNotContains != nil {
			continue
		}
		filtered = append(filtered, assertion)
	}
	return filtered
}

func sortedGoTestAssertions(goTests map[goTestKey]GoTestPassAssertion) []ContractAssertion {
	if len(goTests) == 0 {
		return nil
	}
	keys := make([]goTestKey, 0, len(goTests))
	for k := range goTests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].pkg != keys[j].pkg {
			return keys[i].pkg < keys[j].pkg
		}
		return keys[i].testName < keys[j].testName
	})

	var assertions []ContractAssertion
	for _, key := range keys {
		assertion := goTests[key]
		copy := assertion
		assertions = append(assertions, ContractAssertion{GoTestPass: &copy})
	}
	return assertions
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
