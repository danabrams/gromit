package provider

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func prepareCodexEnv() ([]string, string, error) {
	env := subprocessEnvFn()
	// Don't leak harness CI mode into codex subprocesses. CI mode can suppress
	// runtime usage telemetry, which breaks benchmark measurements.
	env = removeEnvKey(env, "CODEX_CI")
	configuredCodexHome, ok := os.LookupEnv("CODEX_HOME")
	if !ok {
		return env, "", nil
	}
	codexHome, err := ResolveCodexHomePath(configuredCodexHome)
	if err != nil {
		return nil, "", fmt.Errorf("resolving CODEX_HOME: %w", err)
	}
	if err := os.MkdirAll(codexHome, 0755); err != nil {
		fallback, resolveErr := resolveSafeCodexHome()
		if resolveErr != nil || fallback == codexHome {
			return nil, "", fmt.Errorf("ensuring CODEX_HOME exists (%s): %w", codexHome, err)
		}
		if mkFallbackErr := os.MkdirAll(fallback, 0755); mkFallbackErr != nil {
			return nil, "", fmt.Errorf("ensuring CODEX_HOME exists (%s): %w", codexHome, err)
		}
		codexHome = fallback
	}
	env = upsertEnv(env, "CODEX_HOME", codexHome)
	return env, codexHome, nil
}

func resolveSafeCodexHome() (string, error) {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".codex"), nil
	}
	cwd, err := os.Getwd()
	if err != nil || strings.TrimSpace(cwd) == "" {
		return "", fmt.Errorf("resolving fallback CODEX_HOME: %w", err)
	}
	return filepath.Join(cwd, ".codex-home"), nil
}

func isUnderTempDir(path string) bool {
	temp := filepath.Clean(os.TempDir())
	cleaned := filepath.Clean(path)
	if cleaned == temp {
		return true
	}
	rel, err := filepath.Rel(temp, cleaned)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func upsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func removeEnvKey(env []string, key string) []string {
	if len(env) == 0 {
		return env
	}
	prefix := key + "="
	filtered := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

func classifyCodexFailure(exitCode int, stdout, stderr string) string {
	text := stdout + "\n" + stderr
	extras := FailurePatterns{
		Startup: []string{
			"failed to create stdin pipe",
			"failed to create stdout pipe",
			"timed out waiting for first event",
		},
		Transport: []string{
			"stream disconnected",
			"temporary failure in name resolution",
			"name or service not known",
			"temporarily unavailable",
			"econnreset",
			"reconnecting",
		},
	}
	return classifyFailureWithCommonPatterns(exitCode, text, extras)
}

func isTransientCodexFailure(failureCategory string) bool {
	switch failureCategory {
	case FailureCategoryTransportDisconnect, FailureCategoryRateLimited:
		return true
	default:
		return false
	}
}

func shouldRetryCodexAttempt(result *Result, attempt int) bool {
	if result == nil {
		return false
	}
	if attempt >= codexTransientRetryMax {
		return false
	}
	return isTransientCodexFailure(result.FailureCategory)
}

func shouldRetryCodexStartError(err error, attempt int) bool {
	if err == nil || attempt >= codexTransientRetryMax {
		return false
	}
	if errors.Is(err, syscall.EAGAIN) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "resource temporarily unavailable")
}

func codexRetryBackoff(attempt int) time.Duration {
	switch attempt {
	case 0:
		return codexRetryBackoffFirst
	case 1:
		return codexRetryBackoffSecond
	default:
		return codexRetryBackoffDefault
	}
}

func buildCodexDiagnostics(args []string, codexHome, stderr string) string {
	sb := &strings.Builder{}
	sb.WriteString("codex_args=")
	sb.WriteString(strings.Join(args, " "))
	sb.WriteString(" codex_home=")
	if strings.TrimSpace(codexHome) == "" {
		sb.WriteString("unset")
	} else {
		sb.WriteString(codexHome)
	}
	head, tail := splitHeadTail(stderr, 2048)
	sb.WriteString(" stderr_head=")
	sb.WriteString(head)
	sb.WriteString(" stderr_tail=")
	sb.WriteString(tail)
	return sb.String()
}

func splitHeadTail(s string, n int) (string, string) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "empty", "empty"
	}
	if len(trimmed) <= n {
		return trimmed, trimmed
	}
	return trimmed[:n] + "...[truncated]", "...[truncated]" + trimmed[len(trimmed)-n:]
}

func codexDebugf(output io.Writer, format string, args ...interface{}) {
	if !codexDebugEnabled() {
		return
	}
	msg := fmt.Sprintf(format, args...)
	line := "\n[codex-debug] " + msg + "\n"
	if output != nil {
		_, _ = io.WriteString(output, line)
		return
	}
	_, _ = io.WriteString(os.Stderr, line)
}

func codexDebugEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("GROMIT_CODEX_DEBUG"))
	if raw == "" {
		return false
	}
	if b, err := strconv.ParseBool(raw); err == nil {
		return b
	}
	switch strings.ToLower(raw) {
	case "1", "on", "yes", "y":
		return true
	default:
		return false
	}
}
