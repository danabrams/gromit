//go:build contract

package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/test/testutil"
)

func TestMigrationCompatibilityContract_OldConfigExecutionParity(t *testing.T) {
	legacy := runMigrationFixture(t, "legacy_run.yaml")
	explicit := runMigrationFixture(t, "new_explicit_run.yaml")

	if legacy.ClaudeCalls != 1 || explicit.ClaudeCalls != 1 {
		t.Fatalf("claude call count legacy/new = %d/%d, want 1/1", legacy.ClaudeCalls, explicit.ClaudeCalls)
	}
	if legacy.Model != explicit.Model {
		t.Fatalf("resolved build model legacy/new = %q/%q, want parity", legacy.Model, explicit.Model)
	}
}

func TestMigrationCompatibilityContract_NewConfigExplicitResolution(t *testing.T) {
	cfgPath := filepath.Join(fixturesDir, "migration", "new_explicit_resolution.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%q) error = %v", cfgPath, err)
	}

	resolved := cfg.ResolveCompatibilityContext()
	if resolved.Profile.Source != config.CompatibilitySourceExplicit {
		t.Fatalf("profile source = %q, want %q", resolved.Profile.Source, config.CompatibilitySourceExplicit)
	}
	if resolved.TrackerBackend.Source != config.CompatibilitySourceExplicit {
		t.Fatalf("tracker backend source = %q, want %q", resolved.TrackerBackend.Source, config.CompatibilitySourceExplicit)
	}
	if resolved.MethodologyAdapter.Source != config.CompatibilitySourceExplicit {
		t.Fatalf("methodology adapter source = %q, want %q", resolved.MethodologyAdapter.Source, config.CompatibilitySourceExplicit)
	}
}

func TestMigrationCompatibilityContract_LegacyFixtureEmitsDeprecationWarnings(t *testing.T) {
	legacy := runMigrationFixture(t, "legacy_run.yaml")

	if !strings.Contains(legacy.Stderr, config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults) {
		t.Fatalf("legacy stderr missing marker %q\nstderr:\n%s", config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults, legacy.Stderr)
	}
	if !strings.Contains(legacy.Stderr, config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback) {
		t.Fatalf("legacy stderr missing marker %q\nstderr:\n%s", config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback, legacy.Stderr)
	}
}

func TestMigrationCompatibilityContract_CLIOutputShowsLegacyMarkersOnly(t *testing.T) {
	legacy := runMigrationFixture(t, "legacy_run.yaml")
	explicit := runMigrationFixture(t, "new_explicit_run.yaml")

	if !strings.Contains(legacy.Stdout, config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback) {
		t.Fatalf("legacy stdout missing marker %q\nstdout:\n%s", config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback, legacy.Stdout)
	}
	if strings.Contains(explicit.Stdout, config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback) {
		t.Fatalf("explicit stdout unexpectedly included marker %q\nstdout:\n%s", config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback, explicit.Stdout)
	}
}

func TestMigrationCompatibilityContract_ExplicitFixtureOmitsDeprecationWarnings(t *testing.T) {
	legacy := runMigrationFixture(t, "legacy_run.yaml")
	explicit := runMigrationFixture(t, "new_explicit_run.yaml")

	if !legacy.WarningHas(config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults) {
		t.Fatalf("legacy warnings missing marker %q\nstderr:\n%s", config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults, legacy.Stderr)
	}
	if explicit.WarningHas(config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults) {
		t.Fatalf("explicit warnings unexpectedly included marker %q\nstderr:\n%s", config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults, explicit.Stderr)
	}
}

type migrationRunResult struct {
	ClaudeCalls int
	Model       string
	Stdout      string
	Stderr      string
}

func runMigrationFixture(t *testing.T, fixtureName string) migrationRunResult {
	t.Helper()

	env := setupTestEnv(t)
	applyMigrationConfigFixture(t, env, fixtureName)
	seedMigrationHarnessFiles(t, env)

	beadJSON := map[string]any{
		"id":               "migration-bead-1",
		"title":            "Migration parity task",
		"description":      "Ensure old/new config parity",
		"priority":         1,
		"labels":           []string{},
		"parent":           "",
		"issue_type":       "task",
		"status":           "open",
		"owner":            "",
		"expected_outputs": []string{},
	}
	stateJSON := map[string]any{"beads": []any{beadJSON}, "next_id": 2}
	stateBytes, err := json.Marshal(stateJSON)
	if err != nil {
		t.Fatalf("marshal bead state: %v", err)
	}
	if err := os.WriteFile(env.BDStateFile, stateBytes, 0644); err != nil {
		t.Fatalf("write bd state: %v", err)
	}

	stdout, stderr, exitCode, err := runGromitWithEnv(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("run gromit: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("gromit run exit code = %d\nstdout:\n%s\nstderr:\n%s", exitCode, stdout, stderr)
	}

	calls, err := filterCalls(env, "claude")
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	result := migrationRunResult{ClaudeCalls: len(calls)}
	if len(calls) > 0 {
		result.Model = modelFlagValue(calls[0])
	}
	result.Stdout = stdout
	result.Stderr = stderr
	return result
}

func applyMigrationConfigFixture(t *testing.T, env *testEnv, fixtureName string) {
	t.Helper()
	path := filepath.Join(fixturesDir, "migration", fixtureName)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config fixture %q: %v", fixtureName, err)
	}
	if err := os.WriteFile(filepath.Join(env.Dir, "gromit.yaml"), content, 0644); err != nil {
		t.Fatalf("write gromit.yaml: %v", err)
	}
}

func seedMigrationHarnessFiles(t *testing.T, env *testEnv) {
	t.Helper()

	gromitDir := filepath.Join(env.Dir, ".gromit")
	if err := os.MkdirAll(filepath.Join(gromitDir, "templates"), 0755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gromitDir, "logs"), 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(env.Dir, "CLAUDE.md"), []byte("# Contract\n"), 0644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	buildPrompt := "Task {{.Bead.ID}}"
	if err := os.WriteFile(filepath.Join(gromitDir, "templates", "PROMPT_build.md"), []byte(buildPrompt), 0644); err != nil {
		t.Fatalf("write PROMPT_build.md: %v", err)
	}
	claudeFixture := filepath.Join(env.Dir, "claude_success.txt")
	claudeOutput := "ok\n<stream-event>\n<type>tool_use</type>\n<tool>Bash</tool>\n<content>{\"command\":\"git add . && git commit -m 'done'\"}</content>\n</stream-event>\n"
	if err := os.WriteFile(claudeFixture, []byte(claudeOutput), 0644); err != nil {
		t.Fatalf("write CLAUDE_FIXTURE: %v", err)
	}
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", claudeFixture)
}

func modelFlagValue(call string) string {
	parts := strings.Fields(call)
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "--model" {
			return parts[i+1]
		}
	}
	return ""
}
