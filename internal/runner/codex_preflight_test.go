package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func testCodexConfig() *config.Config {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"openai": {
				Binary: "codex",
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

func TestPreflightCodex_SkipsWhenNoCodexProvider(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"claude": {Binary: "claude"},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	r := &Runner{cfg: cfg}
	if err := r.preflightCodex(context.Background()); err != nil {
		t.Fatalf("preflightCodex() error = %v, want nil", err)
	}
}

func TestPreflightCodex_FailsWhenNetworkDisabled(t *testing.T) {
	t.Setenv("CODEX_SANDBOX_NETWORK_DISABLED", "1")

	r := &Runner{
		cfg: testCodexConfig(),
	}

	err := r.preflightCodex(context.Background())
	if err == nil {
		t.Fatal("preflightCodex() should fail when CODEX_SANDBOX_NETWORK_DISABLED=1")
	}
	if !strings.Contains(err.Error(), "CODEX_SANDBOX_NETWORK_DISABLED") {
		t.Fatalf("expected network-disabled error, got: %v", err)
	}
}

func TestPreflightCodex_FailsWhenDNSLookupFails(t *testing.T) {
	t.Setenv("CODEX_SANDBOX_NETWORK_DISABLED", "")

	r := &Runner{
		cfg: testCodexConfig(),
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return nil, fmt.Errorf("dns fail")
		},
	}

	err := r.preflightCodex(context.Background())
	if err == nil {
		t.Fatal("preflightCodex() should fail on DNS lookup failure")
	}
	if !strings.Contains(err.Error(), "cannot resolve api.openai.com") {
		t.Fatalf("expected DNS resolution error, got: %v", err)
	}
}

func TestPreflightCodex_FailsWhenNotLoggedIn(t *testing.T) {
	t.Setenv("CODEX_SANDBOX_NETWORK_DISABLED", "")
	t.Setenv("CODEX_HOME", "/tmp/codex-home-missing")

	r := &Runner{
		cfg: testCodexConfig(),
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return []string{"1.1.1.1"}, nil
		},
		lookPathFn: func(file string) (string, error) {
			return "/usr/bin/codex", nil
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "Not logged in", "", 0, nil
		},
	}

	err := r.preflightCodex(context.Background())
	if err == nil {
		t.Fatal("preflightCodex() should fail when codex login status is not logged in")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("expected not-logged-in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "CODEX_HOME=/tmp/codex-home-missing") {
		t.Fatalf("expected CODEX_HOME hint in error, got: %v", err)
	}
}

func TestPreflightCodex_PassesWhenAllChecksPass(t *testing.T) {
	t.Setenv("CODEX_SANDBOX_NETWORK_DISABLED", "")
	t.Setenv("CODEX_HOME", "")

	r := &Runner{
		cfg: testCodexConfig(),
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return []string{"1.1.1.1"}, nil
		},
		lookPathFn: func(file string) (string, error) {
			if file != "codex" {
				return "", errors.New("unexpected binary")
			}
			return "/usr/bin/codex", nil
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			if command != "codex login status" {
				return "", "", -1, errors.New("unexpected command")
			}
			return "Logged in using ChatGPT", "", 0, nil
		},
	}

	if err := r.preflightCodex(context.Background()); err != nil {
		t.Fatalf("preflightCodex() error = %v, want nil", err)
	}
}
