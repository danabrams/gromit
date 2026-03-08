package debug

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
)

var ErrSystemicChangeApprovalRequired = errors.New("systemic change requires explicit human approval")

const (
	systemicCategoryPromptFragment = "prompt fragments"
	systemicCategoryGuard          = "guards"
	systemicCategoryProcessRule    = "process rules"
)

// EnforceSystemicChangeGuardrails blocks patch application when the diff touches
// systemic assets unless explicit approval is given via --approve or an
// interactive prompt callback.
func EnforceSystemicChangeGuardrails(codePatch string, approve bool, confirmFn func(prompt string) bool) error {
	categories := detectSystemicPatchCategories(codePatch)
	if len(categories) == 0 {
		return nil
	}

	if approve {
		return nil
	}

	if confirmFn != nil {
		prompt := fmt.Sprintf(
			"Patch modifies %s. Apply systemic changes? (or rerun with --approve)",
			strings.Join(categories, ", "),
		)
		if confirmFn(prompt) {
			return nil
		}
	}

	logBlockedSystemicChange(codePatch, categories)

	return fmt.Errorf(
		"%w: patch modifies %s; pass --approve or confirm interactively",
		ErrSystemicChangeApprovalRequired,
		strings.Join(categories, ", "),
	)
}

func detectSystemicPatchCategories(codePatch string) []string {
	seen := map[string]struct{}{}

	for _, path := range extractPatchPaths(codePatch) {
		normalizedPath := strings.ToLower(strings.TrimSpace(path))
		if normalizedPath == "" {
			continue
		}

		if isPromptFragmentPath(normalizedPath) {
			seen[systemicCategoryPromptFragment] = struct{}{}
		}
		if isGuardPath(normalizedPath) {
			seen[systemicCategoryGuard] = struct{}{}
		}
		if isProcessRulePath(normalizedPath) {
			seen[systemicCategoryProcessRule] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	categories := make([]string, 0, len(seen))
	for category := range seen {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	return categories
}

func extractPatchPaths(codePatch string) []string {
	lines := strings.Split(codePatch, "\n")
	seen := map[string]struct{}{}
	paths := make([]string, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "diff --git "):
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			path := strings.TrimPrefix(fields[3], "b/")
			if path == "" {
				continue
			}
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			paths = append(paths, path)
		case strings.HasPrefix(line, "+++ b/"):
			path := strings.TrimPrefix(line, "+++ b/")
			if path == "" {
				continue
			}
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
	}

	return paths
}

func isPromptFragmentPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))

	if strings.Contains(path, ".gromit/fragments/") || strings.Contains(path, "prompt/fragments/") {
		return true
	}
	if strings.Contains(path, "fragments/") && strings.HasSuffix(base, ".md") {
		return true
	}
	if strings.Contains(path, "prompt") && strings.Contains(path, "fragment") {
		return true
	}
	return false
}

func isGuardPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.Contains(path, "guard/") || strings.Contains(path, "guardrail") || strings.Contains(base, "guard")
}

func isProcessRulePath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if base == "rules.md" || base == "agents.md" {
		return true
	}
	if strings.Contains(path, "process/rule") || strings.Contains(path, "workflow/rule") || strings.Contains(path, "process-rule") {
		return true
	}
	return false
}

func logBlockedSystemicChange(codePatch string, categories []string) {
	if len(categories) == 0 {
		return
	}
	paths := extractPatchPaths(codePatch)
	filesDesc := "none"
	if len(paths) > 0 {
		filesDesc = strings.Join(paths, ", ")
	}
	log.Printf(
		"blocked systemic change: categories=%s files=%s",
		strings.Join(categories, ", "),
		filesDesc,
	)
}
