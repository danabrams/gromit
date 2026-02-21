package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/provider"
)

const (
	codexPreflightDNSHost      = "api.openai.com"
	codexPreflightDNSTimeout   = 4 * time.Second
	codexPreflightLoginTimeout = 8 * time.Second
	codexPreflightProbePattern = ".gromit-codex-home-writecheck-*"
	codexLoginProgram          = "codex"
)

var codexLoginStatusArgs = []string{"login", "status"}

// preflightCodex validates Codex runtime prerequisites before the run loop starts.
// It only runs when a Codex-backed provider is configured.
func (r *Runner) preflightCodex(ctx context.Context) error {
	if r == nil || r.cfg == nil || !r.cfg.HasProviders() {
		return nil
	}

	codexBinaries := r.codexProviderBinaries()
	if len(codexBinaries) == 0 {
		return nil
	}

	if disabled := strings.TrimSpace(os.Getenv("CODEX_SANDBOX_NETWORK_DISABLED")); disabled == "1" || strings.EqualFold(disabled, "true") {
		return fmt.Errorf("codex preflight failed: CODEX_SANDBOX_NETWORK_DISABLED=%q blocks network access; unset it before running codex-backed beads", disabled)
	}

	lookupHost := r.lookupHostFn
	if lookupHost == nil {
		return fmt.Errorf("codex preflight failed: DNS lookup function not configured")
	}

	dnsCtx, cancel := context.WithTimeout(ctx, codexPreflightDNSTimeout)
	defer cancel()
	if _, err := lookupHost(dnsCtx, codexPreflightDNSHost); err != nil {
		return fmt.Errorf("codex preflight failed: cannot resolve %s: %w", codexPreflightDNSHost, err)
	}

	lookPath := r.lookPathFn
	if lookPath == nil {
		return fmt.Errorf("codex preflight failed: binary lookup function not configured")
	}
	for _, bin := range codexBinaries {
		if _, err := lookPath(bin); err != nil {
			return fmt.Errorf("codex preflight failed: binary %q not found in PATH", bin)
		}
	}

	codexHome, err := validatePreflightCodexHome()
	if err != nil {
		return err
	}
	r.log("Codex preflight resolved CODEX_HOME=%s", codexHome)

	loginCtx, loginCancel := context.WithTimeout(ctx, codexPreflightLoginTimeout)
	defer loginCancel()
	stdout, stderr, exitCode, statusErr := r.runArgv(loginCtx, codexLoginProgram, codexLoginStatusArgs, "")
	combined := strings.TrimSpace(strings.TrimSpace(stdout) + "\n" + strings.TrimSpace(stderr))
	if statusErr != nil {
		return fmt.Errorf("codex preflight failed: checking login status: %w (output: %s)", statusErr, summarizeCodexPreflightOutput(combined))
	}
	if exitCode != 0 {
		return fmt.Errorf("codex preflight failed: checking login status exited %d (output: %s)", exitCode, summarizeCodexPreflightOutput(combined))
	}
	if strings.Contains(combined, "Not logged in") {
		return fmt.Errorf("codex preflight failed: codex is not logged in for effective CODEX_HOME=%s; run `codex login` (or set CODEX_HOME to your initialized codex profile directory)", codexHome)
	}

	r.log("Codex preflight passed (providers=%s)", strings.Join(codexBinaries, ","))
	return nil
}

func validatePreflightCodexHome() (string, error) {
	configuredCodexHome := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	codexHome, err := provider.ResolveCodexHomePath(configuredCodexHome)
	if err != nil {
		return "", fmt.Errorf("codex preflight failed: resolving effective CODEX_HOME from CODEX_HOME=%q: %w. Remediation: unset CODEX_HOME or set it to a writable directory outside %q", configuredCodexHome, err, os.TempDir())
	}
	if err := os.MkdirAll(codexHome, 0755); err != nil {
		return "", fmt.Errorf("codex preflight failed: effective CODEX_HOME=%q could not be created: %w. Remediation: set CODEX_HOME to a writable directory (for example %q) and rerun `codex login`", codexHome, err, codexHome)
	}
	if err := probeCodexHomeWrite(codexHome); err != nil {
		return "", fmt.Errorf("%w. Remediation: set CODEX_HOME to a writable directory (resolved effective CODEX_HOME=%q)", err, codexHome)
	}
	return codexHome, nil
}

func (r *Runner) codexProviderBinaries() []string {
	if r == nil || r.cfg == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var bins []string
	for name, def := range r.cfg.Providers {
		n := strings.ToLower(strings.TrimSpace(name))
		bin := strings.TrimSpace(def.Binary)
		isCodexProvider := n == "codex" || n == "openai" || strings.EqualFold(filepath.Base(bin), "codex") || strings.EqualFold(bin, "codex")
		if !isCodexProvider {
			continue
		}
		if bin == "" {
			bin = "codex"
		}
		if _, ok := seen[bin]; ok {
			continue
		}
		seen[bin] = struct{}{}
		bins = append(bins, bin)
	}
	return bins
}

func probeCodexHomeWrite(codexHome string) error {
	probeFile, err := os.CreateTemp(codexHome, codexPreflightProbePattern)
	if err != nil {
		return fmt.Errorf("codex preflight failed: CODEX_HOME path %q is not writable: %w", codexHome, err)
	}
	probePath := probeFile.Name()
	if closeErr := probeFile.Close(); closeErr != nil {
		return fmt.Errorf("codex preflight failed: CODEX_HOME writability check failed to close probe file %q: %w", probePath, closeErr)
	}
	if removeErr := os.Remove(probePath); removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("codex preflight failed: CODEX_HOME writability check failed to clean up probe file %q: %w", probePath, removeErr)
	}
	return nil
}

func summarizeCodexPreflightOutput(s string) string {
	if strings.TrimSpace(s) == "" {
		return "no output"
	}
	const max = 240
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}
