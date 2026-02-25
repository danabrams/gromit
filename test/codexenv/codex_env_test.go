package codexenv

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyFixtureEnvNormalizesPaths(t *testing.T) {
	relFixture := filepath.Join("..", "fixtures", "codex_success.txt")
	absFixture, err := filepath.Abs(relFixture)
	if err != nil {
		t.Fatalf("failed to resolve absolute path for %s: %v", relFixture, err)
	}

	baseEnv := []string{"PATH=/tmp/bin", FixtureEnvVar + "=/old/path"}
	updatedEnv := ApplyFixtureEnv(baseEnv, relFixture)

	var gotFixture string
	count := 0
	for _, entry := range updatedEnv {
		if strings.HasPrefix(entry, FixtureEnvVar+"=") {
			count++
			gotFixture = strings.TrimPrefix(entry, FixtureEnvVar+"=")
		}
	}

	if count != 1 {
		t.Fatalf("expected exactly one %s entry, got %d", FixtureEnvVar, count)
	}
	if gotFixture != absFixture {
		t.Fatalf("%s = %q, want %q", FixtureEnvVar, gotFixture, absFixture)
	}

	if baseEnv[1] != FixtureEnvVar+"=/old/path" {
		t.Fatalf("original env mutated: %v", baseEnv)
	}
}
