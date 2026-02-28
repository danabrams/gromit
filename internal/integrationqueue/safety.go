package integrationqueue

import (
	"strings"
)

// SafetyViolation represents a hard safety violation detected in changed files.
type SafetyViolation struct {
	ViolatedFile string
	ViolationType string
}

// ValidateSafety checks changed files against prohibited runtime/local-state artifact
// patterns from RULES.md. Returns nil if no violations are found, or a SafetyViolation
// if a prohibited artifact is detected.
func ValidateSafety(changedFiles []string) *SafetyViolation {
	for _, file := range changedFiles {
		if v := checkProhibitedPattern(file); v != nil {
			return v
		}
	}
	return nil
}

func checkProhibitedPattern(file string) *SafetyViolation {
	// Check approved paths first (whitelist)
	if strings.HasPrefix(file, "test/fixtures/") {
		return nil
	}
	if strings.HasPrefix(file, ".gromit/reports/curated/") {
		return nil
	}

	// Check prohibited patterns
	if strings.HasPrefix(file, ".dolt/") {
		return &SafetyViolation{ViolatedFile: file, ViolationType: "dolt_artifact"}
	}
	if strings.HasPrefix(file, ".doltcfg/") {
		return &SafetyViolation{ViolatedFile: file, ViolationType: "doltcfg_artifact"}
	}
	if strings.HasPrefix(file, "beads_gromit/") {
		return &SafetyViolation{ViolatedFile: file, ViolationType: "beads_gromit_artifact"}
	}
	if file == ".gromit/state.json" {
		return &SafetyViolation{ViolatedFile: file, ViolationType: "gromit_state_json"}
	}
	if file == ".gromit/stats.json" {
		return &SafetyViolation{ViolatedFile: file, ViolationType: "gromit_stats_json"}
	}
	if file == ".gromit/interactive-state.json" {
		return &SafetyViolation{ViolatedFile: file, ViolationType: "gromit_interactive_state_json"}
	}
	if isLockFile(file) {
		return &SafetyViolation{ViolatedFile: file, ViolationType: "lock_file"}
	}

	return nil
}

func isLockFile(file string) bool {
	lockFileNames := map[string]bool{
		"package-lock.json": true,
		"yarn.lock":         true,
		"pnpm-lock.yaml":    true,
		"go.sum":            true,
		"Cargo.lock":        true,
	}

	basename := file[strings.LastIndex(file, "/")+1:]
	return lockFileNames[basename]
}
