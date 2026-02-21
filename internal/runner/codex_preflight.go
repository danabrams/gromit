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

	dnsCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	if _, err := lookupHost(dnsCtx, "api.openai.com"); err != nil {
		return fmt.Errorf("codex preflight failed: cannot resolve api.openai.com: %w", err)
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

	loginCtx, loginCancel := context.WithTimeout(ctx, 8*time.Second)
	defer loginCancel()
	stdout, stderr, exitCode, statusErr := r.runArgv(loginCtx, "codex", []string{"login", "status"}, "")
	combined := strings.TrimSpace(strings.TrimSpace(stdout) + "\n" + strings.TrimSpace(stderr))
	if statusErr != nil {
		return fmt.Errorf("codex preflight failed: checking login status: %w (output: %s)", statusErr, summarizeCodexPreflightOutput(combined))
	}
	if exitCode != 0 {
		return fmt.Errorf("codex preflight failed: checking login status exited %d (output: %s)", exitCode, summarizeCodexPreflightOutput(combined))
	}
	if strings.Contains(combined, "Not logged in") {
		homeHint := strings.TrimSpace(os.Getenv("CODEX_HOME"))
		if homeHint == "" {
			homeHint = "~/.codex"
		}
		return fmt.Errorf("codex preflight failed: codex is not logged in for CODEX_HOME=%s; run `codex login` (or unset CODEX_HOME if it points to an uninitialized directory)", homeHint)
	}

	codexHome, err := provider.ResolveCodexHome()
	if err != nil {
		return fmt.Errorf("codex preflight failed: resolving CODEX_HOME: %w", err)
	}
	if err := os.MkdirAll(codexHome, 0755); err != nil {
		return fmt.Errorf("codex preflight failed: CODEX_HOME path %q does not exist and could not be created: %w", codexHome, err)
	}
	probeFile, err := os.CreateTemp(codexHome, ".gromit-codex-home-writecheck-*")
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
	r.log("Codex preflight resolved CODEX_HOME=%s", codexHome)

	r.log("Codex preflight passed (providers=%s)", strings.Join(codexBinaries, ","))
	return nil
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
