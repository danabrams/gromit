package contract

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const goTestOutputLimit = 2000

var wsRun = regexp.MustCompile(`\s+`)

// containsNormalized collapses all runs of whitespace in both content and
// pattern to a single space, then checks strings.Contains.
func containsNormalized(content, pattern string) bool {
	normContent := wsRun.ReplaceAllString(content, " ")
	normPattern := wsRun.ReplaceAllString(pattern, " ")
	return strings.Contains(normContent, normPattern)
}

// MatchesPattern reports whether content matches pattern using three strategies
// in order: literal strings.Contains, normalized-whitespace containsNormalized,
// and finally regexp.MatchString. If the pattern is not a valid regex the regex
// step is skipped silently — no error is returned.
func MatchesPattern(content, pattern string) bool {
	if strings.Contains(content, pattern) {
		return true
	}
	if containsNormalized(content, pattern) {
		return true
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(content)
}

// ContractEvaluator abstracts contract assertion evaluation for testability.
type ContractEvaluator interface {
	Evaluate(ctx context.Context, contract *ScenarioContract, workDir string) ([]ContractFailure, error)
}

// DefaultContractEvaluator is the default implementation of ContractEvaluator.
type DefaultContractEvaluator struct{}

// Evaluate checks every assertion in contract against workDir and returns all failures.
// All assertions are checked — no short-circuit on first failure.
// A nil contract returns empty failures.
func (e *DefaultContractEvaluator) Evaluate(ctx context.Context, contract *ScenarioContract, workDir string) ([]ContractFailure, error) {
	if contract == nil {
		return nil, nil
	}
	var failures []ContractFailure
	for _, scenario := range contract.Scenarios {
		for _, a := range scenario.Assertions {
			if f := e.check(ctx, scenario.Name, a, workDir); f != nil {
				failures = append(failures, *f)
			}
		}
	}
	return failures, nil
}

func (e *DefaultContractEvaluator) check(ctx context.Context, scenarioName string, a ContractAssertion, workDir string) *ContractFailure {
	fail := func(assertionType, detail string) *ContractFailure {
		return &ContractFailure{
			ScenarioName:  scenarioName,
			AssertionType: assertionType,
			Details:       detail,
			Assertion:     a,
		}
	}

	switch {
	case a.FileExists != "":
		path := filepath.Join(workDir, a.FileExists)
		if _, err := os.Stat(path); err != nil {
			return fail("file_exists", fmt.Sprintf("file %q does not exist", a.FileExists))
		}

	case a.FileNotExists != "":
		path := filepath.Join(workDir, a.FileNotExists)
		_, err := os.Stat(path)
		if err == nil {
			return fail("file_not_exists", fmt.Sprintf("file %q exists but should not", a.FileNotExists))
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fail("file_not_exists", fmt.Sprintf("cannot stat %q: %v", a.FileNotExists, err))
		}

	case a.FileContains != nil:
		path := filepath.Join(workDir, a.FileContains.Path)
		content, err := os.ReadFile(path)
		if err != nil {
			return fail("file_contains", fmt.Sprintf("cannot read %q: %v", a.FileContains.Path, err))
		}
		if !MatchesPattern(string(content), a.FileContains.Pattern) {
			return fail("file_contains", fmt.Sprintf("pattern %q not found in %q", a.FileContains.Pattern, a.FileContains.Path))
		}

	case a.FileNotContains != nil:
		path := filepath.Join(workDir, a.FileNotContains.Path)
		content, err := os.ReadFile(path)
		if err != nil {
			// Nonexistent file trivially does not contain the pattern.
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fail("file_not_contains", fmt.Sprintf("cannot read %q: %v", a.FileNotContains.Path, err))
		}
		// file_not_contains uses literal-only matching (no regex fallback).
		// Regex fallback is dangerous for negative assertions because patterns
		// like "3." match far more than intended (e.g. any "3" + any char).
		if strings.Contains(string(content), a.FileNotContains.Pattern) || containsNormalized(string(content), a.FileNotContains.Pattern) {
			return fail("file_not_contains", fmt.Sprintf("pattern %q found in %q but should not be", a.FileNotContains.Pattern, a.FileNotContains.Path))
		}

	case a.FileNotModified != "":
		// cmd.Dir targets the worktree, so "HEAD" resolves to the worktree's
		// own HEAD (stored in .git/worktrees/<name>/HEAD), NOT the main repo's
		// HEAD. This is safe even when the main branch advances concurrently.
		cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD", "--", a.FileNotModified)
		cmd.Dir = workDir
		out, err := cmd.Output()
		if err != nil {
			return fail("file_not_modified", fmt.Sprintf("git diff failed for %q: %v", a.FileNotModified, err))
		}
		if strings.TrimSpace(string(out)) != "" {
			return fail("file_not_modified", fmt.Sprintf("file %q has been modified", a.FileNotModified))
		}

	case a.GoTestPass != nil:
		// Validate inputs to prevent false passes from empty fields.
		if a.GoTestPass.Pkg == "" {
			return fail("invalid_assertion", "go_test_pass assertion has empty pkg field")
		}
		if a.GoTestPass.TestName == "" {
			return fail("invalid_assertion", "go_test_pass assertion has empty test_name field")
		}
		// Validate test name format: must start with "Test" and contain only alphanumeric and underscores.
		if !regexp.MustCompile(`^Test[0-9A-Za-z_]*$`).MatchString(a.GoTestPass.TestName) {
			return fail("invalid_assertion", fmt.Sprintf("go_test_pass assertion has invalid test_name format: %q (must start with 'Test' and contain only alphanumeric chars and underscores)", a.GoTestPass.TestName))
		}

		outputArg := fmt.Sprintf("^%s$", a.GoTestPass.TestName)
		cmd := exec.CommandContext(ctx, "go", "test", a.GoTestPass.Pkg, "-run", outputArg, "-count=1", "-timeout", "60s")
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			detail := fmt.Sprintf("go test %s -run %s failed: %v\n%s", a.GoTestPass.Pkg, outputArg, err, truncateGoTestOutput(string(out)))
			return fail("go_test_pass", detail)
		}

	default:
		return fail("invalid_assertion", "assertion has no fields set")
	}

	return nil
}

func truncateGoTestOutput(output string) string {
	if len(output) <= goTestOutputLimit {
		return output
	}
	runes := []rune(output)
	if len(runes) <= goTestOutputLimit {
		return output
	}
	return fmt.Sprintf("%s\n[output truncated]", string(runes[:goTestOutputLimit]))
}
