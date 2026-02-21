package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			if program != "codex" {
				return "", "", -1, fmt.Errorf("unexpected program: %s", program)
			}
			if len(args) != 2 || args[0] != "login" || args[1] != "status" {
				return "", "", -1, fmt.Errorf("unexpected args: %#v", args)
			}
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
	if !strings.Contains(err.Error(), "effective CODEX_HOME=") {
		t.Fatalf("expected effective CODEX_HOME context in error, got: %v", err)
	}
}

func TestPreflightCodex_PassesWhenAllChecksPass(t *testing.T) {
	t.Setenv("CODEX_SANDBOX_NETWORK_DISABLED", "")
	t.Setenv("CODEX_HOME", "")
	var output bytes.Buffer

	r := &Runner{
		cfg:    testCodexConfig(),
		output: &output,
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
			return "", "", -1, errors.New("unexpected shell command runner call")
		},
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			if program != "codex" {
				return "", "", -1, errors.New("unexpected program")
			}
			if len(args) != 2 || args[0] != "login" || args[1] != "status" {
				return "", "", -1, errors.New("unexpected args")
			}
			return "Logged in using ChatGPT", "", 0, nil
		},
	}

	if err := r.preflightCodex(context.Background()); err != nil {
		t.Fatalf("preflightCodex() error = %v, want nil", err)
	}
	if count := strings.Count(output.String(), "Codex preflight resolved CODEX_HOME="); count != 1 {
		t.Fatalf("expected exactly one CODEX_HOME resolution log line, got %d logs:\n%s", count, output.String())
	}
}

func TestPreflightCodex_FailsWhenCodexHomeNotWritable(t *testing.T) {
	t.Setenv("CODEX_SANDBOX_NETWORK_DISABLED", "")
	notDirRoot, err := os.MkdirTemp(".", "codex-home-notdir-*")
	if err != nil {
		t.Fatalf("failed to create fixture directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(notDirRoot) })
	filePath := filepath.Join(notDirRoot, "codex-home-file")
	if err := os.WriteFile(filePath, []byte("not-a-directory"), 0644); err != nil {
		t.Fatalf("failed to create file-backed CODEX_HOME fixture: %v", err)
	}
	t.Setenv("CODEX_HOME", filePath)

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
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			if program != "codex" {
				return "", "", -1, errors.New("unexpected program")
			}
			if len(args) != 2 || args[0] != "login" || args[1] != "status" {
				return "", "", -1, errors.New("unexpected args")
			}
			return "Logged in using ChatGPT", "", 0, nil
		},
	}

	err = r.preflightCodex(context.Background())
	if err == nil {
		t.Fatal("preflightCodex() should fail when CODEX_HOME is not writable/creatable")
	}
	if !strings.Contains(err.Error(), "effective CODEX_HOME=") {
		t.Fatalf("expected CODEX_HOME path failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Remediation:") {
		t.Fatalf("expected remediation guidance, got: %v", err)
	}
}

func TestPreflightCodex_ResolvesTempCodexHomeToSafePath(t *testing.T) {
	t.Setenv("CODEX_SANDBOX_NETWORK_DISABLED", "")
	unsafeTempHome := filepath.Join(os.TempDir(), "codex-unsafe-home")
	t.Setenv("CODEX_HOME", unsafeTempHome)
	var output bytes.Buffer

	r := &Runner{
		cfg:    testCodexConfig(),
		output: &output,
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return []string{"1.1.1.1"}, nil
		},
		lookPathFn: func(file string) (string, error) {
			if file != "codex" {
				return "", errors.New("unexpected binary")
			}
			return "/usr/bin/codex", nil
		},
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			if program != "codex" {
				return "", "", -1, errors.New("unexpected program")
			}
			if len(args) != 2 || args[0] != "login" || args[1] != "status" {
				return "", "", -1, errors.New("unexpected args")
			}
			return "Logged in using ChatGPT", "", 0, nil
		},
	}

	if err := r.preflightCodex(context.Background()); err != nil {
		t.Fatalf("preflightCodex() error = %v, want nil", err)
	}
	out := output.String()
	if count := strings.Count(out, "Codex preflight resolved CODEX_HOME="); count != 1 {
		t.Fatalf("expected exactly one CODEX_HOME resolution log line, got %d logs:\n%s", count, out)
	}
	if strings.Contains(out, unsafeTempHome) {
		t.Fatalf("expected resolved CODEX_HOME to avoid unsafe temp path, logs:\n%s", out)
	}
}
