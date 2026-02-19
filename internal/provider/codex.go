package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	// providerNameCodex is the name identifier for the Codex provider
	providerNameCodex = "codex"
)

// CodexProvider wraps the Codex CLI and implements the Provider interface
type CodexProvider struct {
	binaryPath      string
	flags           []string
	tierToModel     map[string]string
	tierToReasoning map[string]string
	sleepFn         func(context.Context, time.Duration) error
}

const (
	codexTransientRetryMax = 2

	codexRetryBackoffFirst   = 250 * time.Millisecond
	codexRetryBackoffSecond  = 750 * time.Millisecond
	codexRetryBackoffDefault = 1500 * time.Millisecond
)

// Compile-time check to verify CodexProvider implements Provider interface
var _ Provider = (*CodexProvider)(nil)

// NewCodexProvider creates a new CodexProvider with the given configuration
func NewCodexProvider(binaryPath string, flags []string, tierToModel map[string]string) *CodexProvider {
	return &CodexProvider{
		binaryPath:      binaryPath,
		flags:           flags,
		tierToModel:     tierToModel,
		tierToReasoning: map[string]string{},
		sleepFn:         sleepWithContext,
	}
}

// SetReasoningEffort configures per-tier reasoning effort (for example:
// {"high":"high","medium":"medium"}), which is forwarded to Codex via
// `-c model_reasoning_effort="<value>"` when the tier is invoked.
func (cp *CodexProvider) SetReasoningEffort(tierToReasoning map[string]string) {
	if cp == nil {
		return
	}
	cp.tierToReasoning = map[string]string{}
	for tier, effort := range tierToReasoning {
		key := strings.ToLower(strings.TrimSpace(tier))
		val := strings.ToLower(strings.TrimSpace(effort))
		if key == "" || val == "" {
			continue
		}
		cp.tierToReasoning[key] = val
	}
}

// Name returns the provider name "codex"
func (cp *CodexProvider) Name() string {
	return providerNameCodex
}

// ModelForTier returns the model name for a given tier without invoking the LLM
func (cp *CodexProvider) ModelForTier(tier string) string {
	if modelName, ok := cp.tierToModel[tier]; ok {
		return modelName
	}
	return tier
}

// Run executes an LLM invocation with the given prompt and tier.
// Uses --json mode to get structured JSONL output, then extracts the
// agent's text response so callers receive the model's actual content.
func (cp *CodexProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("codex provider is nil")
	}

	model := cp.ModelForTier(tier)
	args := cp.buildCommandArgsForTier(model, tier, true)
	env, effectiveCodexHome, err := prepareCodexEnv()
	if err != nil {
		return nil, err
	}
	result, err := cp.runWithRetry(ctx, func() (*Result, error) {
		return cp.runOnce(ctx, prompt, model, args, env, effectiveCodexHome)
	})
	if err != nil {
		return nil, err
	}
	// Extract agent text from JSONL events so callers get the model's
	// actual response rather than raw JSONL lines.
	if text := extractAgentTextFromJSONL(result.Output); text != "" {
		result.Output = text
	}
	return result, nil
}

// StreamRun executes an LLM invocation with streaming output.
// When EventHandler is non-nil, invokes codex with --json and parses JSONL events.
// Transient failures (transport_disconnect, rate_limited) are retried with bounded backoff.
func (cp *CodexProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("codex provider is nil")
	}

	return cp.runWithRetry(ctx, func() (*Result, error) {
		return cp.streamRunOnce(ctx, prompt, tier, output, handler, onToolCall)
	})
}

// streamRunOnce executes a single streaming invocation attempt.
func (cp *CodexProvider) streamRunOnce(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	model := cp.ModelForTier(tier)
	args := cp.buildStreamCommandArgsForTier(model, tier, handler != nil)
	cmd := execCommandContext(ctx, cp.binaryPath, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	if output != nil {
		fmt.Fprintf(output, "  cmd: %s %s\n", cp.binaryPath, strings.Join(args, " "))
		fmt.Fprintf(output, "  prompt length: %d bytes\n", len(prompt))
	}
	env, effectiveCodexHome, err := prepareCodexEnv()
	if err != nil {
		return nil, err
	}
	cmd.Env = env

	// Set up stdin pipe for prompt delivery
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	startTime := time.Now()

	// Always use JSONL parsing for cost/token tracking. Handler may be nil;
	// processCodexStream guards all handler calls with nil checks.
	codexDebugf(output, "provider debug: StreamRun start model=%s tier=%s args=%q", model, tier, strings.Join(args, " "))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start codex command: %w", err)
	}
	codexDebugf(output, "provider debug: StreamRun cmd started pid=%d", cmd.Process.Pid)

	// Write prompt to stdin in goroutine
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, prompt)
	}()

	// Process the JSONL stream
	codexDebugf(output, "provider debug: processCodexStream begin")
	resultText, usage, streamErrInfo, err := processCodexStream(stdout, output, handler, onToolCall)
	codexDebugf(output, "provider debug: processCodexStream end err=%v result_chars=%d usage_nil=%t error_info_nil=%t", err, len(resultText), usage == nil, streamErrInfo == nil)
	if err != nil {
		codexDebugf(output, "provider debug: waiting for process after stream error")
		cmd.Wait()
		if ctx.Err() != nil {
			return nil, fmt.Errorf("codex command cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("failed to process codex stream: %w", err)
	}

	codexDebugf(output, "provider debug: waiting for cmd.Wait after successful stream parse")
	if err := cmd.Wait(); err != nil {
		codexDebugf(output, "provider debug: cmd.Wait returned err=%v", err)
		if ctx.Err() != nil {
			return nil, fmt.Errorf("codex command cancelled: %w", ctx.Err())
		}
		duration := time.Since(startTime)
		exitCode, _ := cp.extractExitCode(err)
		return &Result{
			Success:           false,
			Output:            resultText,
			Stderr:            stderr.String(),
			Diagnostics:       buildCodexDiagnostics(args, effectiveCodexHome, stderr.String()),
			FailureCategory:   classifyCodexFailure(exitCode, resultText, stderr.String()),
			ExitCode:          exitCode,
			Duration:          duration,
			Model:             model,
			CostUSD:           usageCost(usage),
			InputTokens:       usageInputTokens(usage),
			CachedInputTokens: usageCachedInputTokens(usage),
			OutputTokens:      usageOutputTokens(usage),
		}, nil
	}
	codexDebugf(output, "provider debug: cmd.Wait returned success")

	duration := time.Since(startTime)

	// If the turn ended with an error (e.g. UsageLimitExceeded), report failure
	if streamErrInfo != nil {
		return &Result{
			Success:           false,
			Output:            resultText,
			Diagnostics:       buildCodexDiagnostics(args, effectiveCodexHome, ""),
			FailureCategory:   classifyCodexFailure(0, resultText, ""),
			ExitCode:          0,
			Duration:          duration,
			Model:             model,
			CostUSD:           usageCost(usage),
			InputTokens:       usageInputTokens(usage),
			CachedInputTokens: usageCachedInputTokens(usage),
			OutputTokens:      usageOutputTokens(usage),
		}, nil
	}

	return &Result{
		Success:           true,
		Output:            resultText,
		Diagnostics:       buildCodexDiagnostics(args, effectiveCodexHome, ""),
		ExitCode:          0,
		Duration:          duration,
		Model:             model,
		CostUSD:           usageCost(usage),
		InputTokens:       usageInputTokens(usage),
		CachedInputTokens: usageCachedInputTokens(usage),
		OutputTokens:      usageOutputTokens(usage),
	}, nil
}

func (cp *CodexProvider) runWithRetry(ctx context.Context, run func() (*Result, error)) (*Result, error) {
	var last *Result
	for attempt := 0; attempt <= codexTransientRetryMax; attempt++ {
		result, err := run()
		if err != nil {
			return nil, err
		}
		last = result
		if result == nil || result.Success {
			return result, nil
		}
		if !shouldRetryCodexAttempt(result, attempt) {
			return result, nil
		}
		if sleepErr := cp.sleepFn(ctx, codexRetryBackoff(attempt)); sleepErr != nil {
			return result, nil
		}
	}
	return last, nil
}

func (cp *CodexProvider) runOnce(ctx context.Context, prompt, model string, args, env []string, effectiveCodexHome string) (*Result, error) {
	cmd := execCommandContext(ctx, cp.binaryPath, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start codex command: %w", err)
	}

	go func() {
		defer stdin.Close()
		_, _ = io.WriteString(stdin, prompt)
	}()

	err = cmd.Wait()
	duration := time.Since(startTime)

	if ctx.Err() != nil {
		return nil, fmt.Errorf("codex command cancelled: %w", ctx.Err())
	}

	output := stdout.String()
	exitCode, err := cp.extractExitCode(err)
	if err != nil {
		return nil, err
	}

	return &Result{
		Success:         exitCode == 0,
		Output:          output,
		Stderr:          stderr.String(),
		Diagnostics:     buildCodexDiagnostics(args, effectiveCodexHome, stderr.String()),
		FailureCategory: classifyCodexFailure(exitCode, output, stderr.String()),
		ExitCode:        exitCode,
		Duration:        duration,
		Model:           model,
	}, nil
}

func prepareCodexEnv() ([]string, string, error) {
	env := os.Environ()
	codexHome, ok := os.LookupEnv("CODEX_HOME")
	if !ok || strings.TrimSpace(codexHome) == "" {
		return env, "", nil
	}
	codexHome = strings.TrimSpace(codexHome)
	if isUnderTempDir(codexHome) {
		safeHome, err := resolveSafeCodexHome()
		if err != nil {
			return nil, "", fmt.Errorf("resolving safe CODEX_HOME from temp path (%s): %w", codexHome, err)
		}
		codexHome = safeHome
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

func classifyCodexFailure(exitCode int, stdout, stderr string) string {
	if exitCode == 0 {
		return FailureCategoryNone
	}
	text := strings.ToLower(strings.TrimSpace(stdout + "\n" + stderr))
	if text == "" {
		return FailureCategoryOther
	}
	authPatterns := []string{
		"unauthorized",
		"invalid api key",
		"authentication",
		"forbidden",
	}
	for _, p := range authPatterns {
		if strings.Contains(text, p) {
			return FailureCategoryAuth
		}
	}
	transportPatterns := []string{
		"stream disconnected",
		"could not resolve host",
		"temporary failure in name resolution",
		"name or service not known",
		"connection reset",
		"connection refused",
		"connection timed out",
		"timeout",
		"temporarily unavailable",
		"internal server error",
		"service unavailable",
		"broken pipe",
		"econnreset",
		"reconnecting",
	}
	for _, p := range transportPatterns {
		if strings.Contains(text, p) {
			return FailureCategoryTransportDisconnect
		}
	}
	ratePatterns := []string{"rate limit", "too many requests", "quota exceeded", "429", "503"}
	for _, p := range ratePatterns {
		if strings.Contains(text, p) {
			return FailureCategoryRateLimited
		}
	}
	return FailureCategoryOther
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

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
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

// RunValidation constructs a validation prompt and runs it via Codex.
// Uses the same prompt pattern as ClaudeProvider for consistency.
func (cp *CodexProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("codex provider is nil")
	}

	// Validate commands
	if err := ValidateCommands(commands); err != nil {
		return nil, err
	}

	// Build validation prompt
	prompt := BuildValidationPrompt(commands, workDir)

	// Run the validation prompt
	return cp.Run(ctx, prompt, tier)
}

// IsUsageLimitError detects Codex-specific usage limit errors.
// Checks for output containing "usage limit", "rate limit", or "quota exceeded"
// (case-insensitive) with a non-success result.
func (cp *CodexProvider) IsUsageLimitError(result *Result, err error) bool {
	if result == nil {
		return false
	}

	// Must be a failure to be a usage limit error
	if result.Success {
		return false
	}

	// Check for usage limit keywords (case-insensitive)
	outputLower := strings.ToLower(result.Output)
	keywords := []string{"usage limit", "rate limit", "quota exceeded"}
	for _, keyword := range keywords {
		if strings.Contains(outputLower, keyword) {
			return true
		}
	}

	return false
}

// IsValidationPassed delegates to the shared helper function.
func (cp *CodexProvider) IsValidationPassed(result *Result) bool {
	return IsValidationPassed(result)
}

// IsScopeTooLarge delegates to the shared helper function.
func (cp *CodexProvider) IsScopeTooLarge(result *Result) (bool, string) {
	return IsScopeTooLarge(result)
}

// buildCommandArgs constructs the command arguments for the Codex CLI invocation.
// Returns: ['exec', user_flags..., '--full-auto'|”, '--skip-git-repo-check', '--color', 'never', '--model', model, [--json], '-']
// If user flags include --dangerously-bypass-approvals-and-sandbox, --full-auto is
// omitted because the two flags are mutually exclusive in the Codex CLI.
func (cp *CodexProvider) buildCommandArgs(model string, jsonMode bool) []string {
	return cp.buildCommandArgsForTier(model, "", jsonMode)
}

// buildStreamCommandArgs constructs command arguments for streaming invocations.
// Always includes --json to ensure cost/token data is captured from JSONL events.
// The jsonMode parameter is ignored; JSON mode is always enabled for streaming.
func (cp *CodexProvider) buildStreamCommandArgs(model string, _ bool) []string {
	return cp.buildStreamCommandArgsForTier(model, "", true)
}

func (cp *CodexProvider) buildCommandArgsForTier(model, tier string, jsonMode bool) []string {
	return cp.buildExecCommandArgs(model, tier, "never", jsonMode)
}

func (cp *CodexProvider) buildStreamCommandArgsForTier(model, tier string, _ bool) []string {
	return cp.buildExecCommandArgs(model, tier, "auto", true)
}

func (cp *CodexProvider) buildExecCommandArgs(model, tier, colorMode string, jsonMode bool) []string {
	args := make([]string, 0, len(cp.flags)+12)
	args = append(args, "exec")
	args = append(args, cp.flags...)

	if !hasBypassApprovalsAndSandbox(cp.flags) {
		args = append(args, "--full-auto")
	}

	args = append(args, "--skip-git-repo-check")
	args = append(args, "--color", colorMode)
	args = append(args, "--model", model)
	if effort, ok := cp.reasoningEffortForTier(tier); ok && !hasReasoningEffortConfig(cp.flags) {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", effort))
	}
	if jsonMode {
		args = append(args, "--json")
	}
	args = append(args, "-")
	return args
}

func (cp *CodexProvider) reasoningEffortForTier(tier string) (string, bool) {
	if cp == nil || len(cp.tierToReasoning) == 0 {
		return "", false
	}
	effort, ok := cp.tierToReasoning[strings.ToLower(strings.TrimSpace(tier))]
	if !ok || strings.TrimSpace(effort) == "" {
		return "", false
	}
	return effort, true
}

func hasBypassApprovalsAndSandbox(flags []string) bool {
	for _, f := range flags {
		if f == "--dangerously-bypass-approvals-and-sandbox" {
			return true
		}
	}
	return false
}

func hasReasoningEffortConfig(flags []string) bool {
	for i := 0; i < len(flags); i++ {
		flag := flags[i]
		if strings.HasPrefix(flag, "--config") || strings.HasPrefix(flag, "-c") {
			if strings.Contains(flag, "model_reasoning_effort") {
				return true
			}
			if i+1 < len(flags) && strings.Contains(flags[i+1], "model_reasoning_effort") {
				return true
			}
		}
	}
	return false
}

// extractExitCode extracts the exit code from a command execution error.
// Returns 0 for success, the exit code for ExitError, or an error if the command
// failed to start.
func (cp *CodexProvider) extractExitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), nil
	}

	return 0, fmt.Errorf("failed to execute codex command: %w", err)
}

// codexUsage represents token usage data from Codex turn.completed events
type codexUsage struct {
	InputTokens       int     `json:"input_tokens"`
	CachedInputTokens int     `json:"cached_input_tokens"`
	OutputTokens      int     `json:"output_tokens"`
	TotalCostUSD      float64 `json:"total_cost_usd,omitempty"`
}

// codexErrorInfo represents error information from Codex turn.completed events
type codexErrorInfo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// extractAgentTextFromJSONL scans JSONL output for item.completed events
// with type "agent_message" and returns the last agent's text response.
// This is the non-streaming counterpart to processCodexStream's text extraction.
func extractAgentTextFromJSONL(output string) string {
	var last string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event codexEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item != nil && event.Item.Type == "agent_message" {
			last = event.Item.Text
		}
	}
	return last
}

// codexItem represents an item from Codex item.started or item.completed events
type codexItem struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Command  string `json:"command"`
	Path     string `json:"path"`
	ToolName string `json:"tool_name"`
}

// codexDelta represents incremental text from Codex delta events
type codexDelta struct {
	Text string `json:"text"`
}

// codexEvent represents a top-level Codex JSONL event
type codexEvent struct {
	Type         string          `json:"type"`
	Item         *codexItem      `json:"item,omitempty"`
	Delta        *codexDelta     `json:"delta,omitempty"`
	Status       string          `json:"status,omitempty"`
	Usage        *codexUsage     `json:"usage,omitempty"`
	ErrorInfo    *codexErrorInfo `json:"error,omitempty"`
	TotalCostUSD float64         `json:"total_cost_usd,omitempty"`
}

// processCodexStream reads Codex JSONL events from reader, converts them to StreamEvent format,
// and calls handlers for each event. Returns the final result text (from last agent_message),
// token usage data (from turn.completed), error info (from failed turn.completed), and any error encountered.
func processCodexStream(reader io.Reader, output io.Writer, handler EventHandler, toolHandler ToolCallHandler) (string, *codexUsage, *codexErrorInfo, error) {
	scanner := bufio.NewScanner(reader)
	const maxTokenSize = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxTokenSize)
	var lastAgentText string
	var usage *codexUsage
	var errInfo *codexErrorInfo
	var sawDeltas bool
	var eventCount int
	started := time.Now()
	lastEvent := started
	var lastEventUnixNano int64 = started.UnixNano()
	var eventsSeen int64

	stopWatchdog := make(chan struct{})
	if codexDebugEnabled() {
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					last := time.Unix(0, atomic.LoadInt64(&lastEventUnixNano))
					idle := time.Since(last).Round(time.Millisecond)
					codexDebugf(output, "provider debug: scanner watchdog events=%d idle=%s", atomic.LoadInt64(&eventsSeen), idle)
				case <-stopWatchdog:
					return
				}
			}
		}()
	}
	defer close(stopWatchdog)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		eventCount++
		lastEvent = time.Now()
		atomic.StoreInt64(&lastEventUnixNano, lastEvent.UnixNano())
		atomic.StoreInt64(&eventsSeen, int64(eventCount))

		var event codexEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Skip malformed lines silently
			codexDebugf(output, "provider debug: malformed json event ignored len=%d", len(line))
			continue
		}
		if eventCount <= 5 || eventCount%50 == 0 {
			codexDebugf(output, "provider debug: stream event #%d type=%s", eventCount, event.Type)
		}

		// Handle different event types
		switch event.Type {
		case "thread.started":
			if handler != nil {
				streamEvent := map[string]interface{}{
					"type": "system",
				}
				eventJSON, _ := json.Marshal(streamEvent)
				handler(eventJSON)
			}

		case "item.started":
			if event.Item != nil && toolHandler != nil {
				var toolName, filePath string
				switch event.Item.Type {
				case "command_execution":
					toolName = "Bash"
					filePath = event.Item.Command
				case "file_change":
					toolName = "Write"
					filePath = event.Item.Path
				case "mcp_tool_call":
					toolName = event.Item.ToolName
					filePath = ""
				}

				if toolName != "" {
					toolHandler(ToolEvent{
						ToolName:  toolName,
						FilePath:  filePath,
						Timestamp: time.Now(),
					})
				}
			}

		case "item.agentMessage.delta":
			// Stream incremental text to terminal in real-time
			if event.Delta != nil && event.Delta.Text != "" {
				sawDeltas = true
				if output != nil {
					output.Write([]byte(event.Delta.Text))
				}
			}

		case "item.completed":
			if event.Item != nil && event.Item.Type == "agent_message" {
				// Extract agent text (used as final result)
				lastAgentText = event.Item.Text

				// Write to output only if no delta events were seen
				// (deltas already streamed text incrementally)
				if !sawDeltas && output != nil && event.Item.Text != "" {
					output.Write([]byte(event.Item.Text))
				}

				// Emit assistant event
				if handler != nil {
					streamEvent := map[string]interface{}{
						"type": "assistant",
						"message": map[string]interface{}{
							"content": []map[string]interface{}{
								{
									"type": "text",
									"text": event.Item.Text,
								},
							},
						},
					}
					eventJSON, _ := json.Marshal(streamEvent)
					handler(eventJSON)
				}
			}

		case "result":
			// Extract usage from native result events (matches Claude's reporting path).
			// Some codex provider versions report token usage in a "result" event with
			// a nested "usage" field rather than via "turn.completed" events.
			if event.Usage != nil {
				if usage == nil {
					usage = event.Usage
				}
				if usage.TotalCostUSD == 0 && event.TotalCostUSD > 0 {
					usage.TotalCostUSD = event.TotalCostUSD
				}
			}

		case "turn.completed":
			// Extract usage data
			if event.Usage != nil {
				usage = event.Usage
				// Prefer cost nested in usage, but accept top-level for compatibility.
				if usage.TotalCostUSD == 0 && event.TotalCostUSD > 0 {
					usage.TotalCostUSD = event.TotalCostUSD
				}
			}

			// Capture error info from failed turns
			if event.ErrorInfo != nil {
				errInfo = event.ErrorInfo
			}

			// Emit error event for failure conditions
			if handler != nil && event.ErrorInfo != nil {
				streamEvent := map[string]interface{}{
					"type":    "error",
					"subtype": event.ErrorInfo.Type,
					"message": event.ErrorInfo.Message,
				}
				eventJSON, _ := json.Marshal(streamEvent)
				handler(eventJSON)
			}

			// Emit result event with token usage
			if handler != nil && event.Usage != nil {
				totalCostUSD := event.TotalCostUSD
				if totalCostUSD == 0 && event.Usage.TotalCostUSD > 0 {
					totalCostUSD = event.Usage.TotalCostUSD
				}
				streamEvent := map[string]interface{}{
					"type":           "result",
					"total_cost_usd": totalCostUSD,
					"input_tokens":   event.Usage.InputTokens,
					"output_tokens":  event.Usage.OutputTokens,
				}
				eventJSON, _ := json.Marshal(streamEvent)
				handler(eventJSON)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		codexDebugf(output, "provider debug: scanner err after %d events and %s since last event: %v", eventCount, time.Since(lastEvent).Round(time.Millisecond), err)
		return "", nil, nil, err
	}
	codexDebugf(output, "provider debug: scanner completed events=%d duration=%s last_event_ago=%s", eventCount, time.Since(started).Round(time.Millisecond), time.Since(lastEvent).Round(time.Millisecond))

	return lastAgentText, usage, errInfo, nil
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

func usageCost(usage *codexUsage) float64 {
	if usage == nil {
		return 0
	}
	return usage.TotalCostUSD
}

func usageInputTokens(usage *codexUsage) int {
	if usage == nil {
		return 0
	}
	return usage.InputTokens
}

func usageCachedInputTokens(usage *codexUsage) int {
	if usage == nil {
		return 0
	}
	return usage.CachedInputTokens
}

func usageOutputTokens(usage *codexUsage) int {
	if usage == nil {
		return 0
	}
	return usage.OutputTokens
}
