package toolcalls

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCodexFixtureEnv(t *testing.T) {
	env := []string{"PATH=/tmp/bin", "HOME=/tmp/home"}
	got := ApplyCodexFixtureEnv(env, "/tmp/codex-fixture.txt")

	found := false
	for _, v := range got {
		if v == "CODEX_FIXTURE=/tmp/codex-fixture.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("CODEX_FIXTURE was not set in env: %v", got)
	}
}

func TestApplyCodexFailEnv(t *testing.T) {
	env := []string{"PATH=/tmp/bin", "HOME=/tmp/home"}
	got := ApplyCodexFailEnv(env, "42")

	found := false
	for _, v := range got {
		if v == "CODEX_FAIL=42" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("CODEX_FAIL was not set in env: %v", got)
	}
}

func TestApplyCodexDelayEnv(t *testing.T) {
	env := []string{"PATH=/tmp/bin", "HOME=/tmp/home"}
	got := ApplyCodexDelayEnv(env, "5s")

	found := false
	for _, v := range got {
		if v == "CODEX_DELAY=5s" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("CODEX_DELAY was not set in env: %v", got)
	}
}

func TestApplyCodexEnv_ComposesMultipleHelpers(t *testing.T) {
	env := []string{"PATH=/tmp/bin", "HOME=/tmp/home"}

	// Apply all three helpers in sequence
	env = ApplyCodexFixtureEnv(env, "/tmp/codex-fixture.txt")
	env = ApplyCodexFailEnv(env, "42")
	env = ApplyCodexDelayEnv(env, "5s")

	// Verify all three are set
	foundFixture := false
	foundFailure := false
	foundDelay := false

	for _, v := range env {
		if v == "CODEX_FIXTURE=/tmp/codex-fixture.txt" {
			foundFixture = true
		}
		if v == "CODEX_FAIL=42" {
			foundFailure = true
		}
		if v == "CODEX_DELAY=5s" {
			foundDelay = true
		}
	}

	if !foundFixture {
		t.Fatalf("CODEX_FIXTURE was not set in env: %v", env)
	}
	if !foundFailure {
		t.Fatalf("CODEX_FAIL was not set in env: %v", env)
	}
	if !foundDelay {
		t.Fatalf("CODEX_DELAY was not set in env: %v", env)
	}
}

func TestApplyCodexFixtureEnv_ReplacesExistingValue(t *testing.T) {
	env := []string{"PATH=/tmp/bin", "CODEX_FIXTURE=/old/path"}
	got := ApplyCodexFixtureEnv(env, "/new/path")

	count := 0
	for _, v := range got {
		if strings.HasPrefix(v, "CODEX_FIXTURE=") {
			count++
			if v != "CODEX_FIXTURE=/new/path" {
				t.Fatalf("expected CODEX_FIXTURE=/new/path, got %q", v)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 CODEX_FIXTURE in env, got %d", count)
	}
}

func TestApplyCodexFixtureEnv_NormalizesRelativePaths(t *testing.T) {
	relFixture := filepath.Join("..", "fixtures", "codex_success.txt")
	absFixture, err := filepath.Abs(relFixture)
	if err != nil {
		t.Fatalf("failed to resolve absolute path for %s: %v", relFixture, err)
	}

	baseEnv := []string{"PATH=/tmp/bin", "CODEX_FIXTURE=/old/path"}
	updatedEnv := ApplyCodexFixtureEnv(baseEnv, relFixture)

	var gotFixture string
	for _, entry := range updatedEnv {
		if strings.HasPrefix(entry, "CODEX_FIXTURE=") {
			gotFixture = strings.TrimPrefix(entry, "CODEX_FIXTURE=")
			break
		}
	}

	if gotFixture != absFixture {
		t.Fatalf("CODEX_FIXTURE = %q, want %q", gotFixture, absFixture)
	}

	if baseEnv[1] != "CODEX_FIXTURE=/old/path" {
		t.Fatalf("ApplyCodexFixtureEnv mutated original env: got %v", baseEnv)
	}
}
