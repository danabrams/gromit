package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/procutil"
)

const (
	// providerNameCodex is the name identifier for the Codex provider
	providerNameCodex = "codex"
)

var codexKillDescendantsOnCancelFn = procutil.KillDescendantsOnCancel

// ModelPricing holds per-million-token USD rates for a specific model.
// Used as a fallback when the provider does not emit total_cost_usd in the stream.
type ModelPricing struct {
	InputCostPerMillion  float64 // USD per 1M input tokens
	OutputCostPerMillion float64 // USD per 1M output tokens
}

// CodexProvider wraps the Codex CLI and implements the Provider interface
type CodexProvider struct {
	binaryPath           string
	flags                []string
	tierToModel          map[string]string
	tierToReasoning      map[string]string
	tierToMaxInputTokens map[string]int
	modelPricing         map[string]ModelPricing
	sleepFn              func(context.Context, time.Duration) error
	cacheAdapter         CacheAdapter
}

const (
	codexTransientRetryMax = 2

	codexRetryBackoffFirst   = 250 * time.Millisecond
	codexRetryBackoffSecond  = 750 * time.Millisecond
	codexRetryBackoffDefault = 1500 * time.Millisecond
	codexCommandWaitDelay    = 100 * time.Millisecond
	codexProcessCapacityWait = 1500 * time.Millisecond
)

// Compile-time check to verify CodexProvider implements Provider interface
var _ Provider = (*CodexProvider)(nil)

// ResolveCodexHomePath returns the effective CODEX_HOME path for the provided
// CODEX_HOME value. Empty values or values under the system temp directory are
// rewritten to a safe fallback path.
func ResolveCodexHomePath(codexHome string) (string, error) {
	codexHome = strings.TrimSpace(codexHome)
	if codexHome == "" || isUnderTempDir(codexHome) {
		safeHome, err := resolveSafeCodexHome()
		if err != nil {
			return "", err
		}
		codexHome = safeHome
	}
	cleaned := filepath.Clean(codexHome)
	if isUnderTempDir(cleaned) {
		return "", fmt.Errorf("resolved CODEX_HOME is unsafe (under temp directory): %s", cleaned)
	}
	return cleaned, nil
}

// ResolveCodexHome returns the effective CODEX_HOME path from environment.
func ResolveCodexHome() (string, error) {
	return ResolveCodexHomePath(os.Getenv("CODEX_HOME"))
}

// NewCodexProvider creates a new CodexProvider with the given configuration
func NewCodexProvider(binaryPath string, flags []string, tierToModel map[string]string) *CodexProvider {
	return &CodexProvider{
		binaryPath:           binaryPath,
		flags:                flags,
		tierToModel:          tierToModel,
		tierToReasoning:      map[string]string{},
		tierToMaxInputTokens: map[string]int{},
		sleepFn:              procutil.SleepWithContext,
		cacheAdapter:         NewNoopCacheAdapter(),
	}
}

// SetReasoningEffort configures per-tier reasoning effort (for example:
// {"high":"high","medium":"medium"}), which is forwarded to Codex via
// `-c model_reasoning_effort=<value>` when the tier is invoked.
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

// SetModelPricing configures per-model token pricing for cost fallback.
// When Codex does not emit total_cost_usd in its stream (e.g. ChatGPT accounts),
// CostUSD is computed as: inputTokens/1M × InputCostPerMillion + outputTokens/1M × OutputCostPerMillion.
// If the model is not in the map, CostUSD remains 0.
func (cp *CodexProvider) SetModelPricing(pricing map[string]ModelPricing) {
	if cp == nil {
		return
	}
	cp.modelPricing = make(map[string]ModelPricing, len(pricing))
	for model, p := range pricing {
		cp.modelPricing[model] = p
	}
}

// computeCostUSD returns cost from stream usage. If total_cost_usd is zero but
// tokens are present and model pricing is configured, falls back to token × rate.
func (cp *CodexProvider) computeCostUSD(usage *codexUsage, model string) float64 {
	if c := usageCost(usage); c > 0 {
		return c
	}
	if usage == nil || cp == nil || len(cp.modelPricing) == 0 {
		return 0
	}
	p, ok := cp.modelPricing[model]
	if !ok {
		return 0
	}
	return float64(usage.InputTokens)/1_000_000*p.InputCostPerMillion +
		float64(usage.OutputTokens)/1_000_000*p.OutputCostPerMillion
}

// SetMaxInputTokens configures per-tier maximum input token thresholds.
// When a tier exceeds this threshold during invocation, a warning is emitted.
func (cp *CodexProvider) SetMaxInputTokens(tier string, maxTokens int) {
	if cp == nil {
		return
	}
	key := strings.ToLower(strings.TrimSpace(tier))
	if key == "" || maxTokens <= 0 {
		return
	}
	cp.tierToMaxInputTokens[key] = maxTokens
}

// MaxInputTokensForTier returns the configured maximum input token threshold for a tier,
// or 0 if no limit is set.
func (cp *CodexProvider) MaxInputTokensForTier(tier string) int {
	if cp == nil {
		return 0
	}
	key := strings.ToLower(strings.TrimSpace(tier))
	return cp.tierToMaxInputTokens[key]
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

// CacheAdapter returns the cache adapter configured for this provider.
func (cp *CodexProvider) CacheAdapter() CacheAdapter {
	if cp == nil || cp.cacheAdapter == nil {
		return NewNoopCacheAdapter()
	}
	return cp.cacheAdapter
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
	reasoningEffort := reasoningEffortFromArgs(args)
	cmd := execCommandContext(ctx, cp.binaryPath, args...)
	cmd.WaitDelay = codexCommandWaitDelay
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

	if waitErr := procutil.WaitForProcessCapacity(ctx, codexProcessCapacityWait); waitErr != nil {
		return nil, fmt.Errorf("waiting for process capacity: %w", waitErr)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start codex command: %w", err)
	}
	// Terminate any spawned subtree when the context is cancelled.
	codexKillDescendantsOnCancelFn(ctx, cmd)
	defer reapProcessGroupFn(cmd)
	codexDebugf(output, "provider debug: StreamRun cmd started pid=%d", cmd.Process.Pid)

	// Write prompt to stdin in goroutine
	go func() {
		defer stdin.Close()
		if _, err := io.WriteString(stdin, prompt); err != nil {
			codexDebugf(output, "provider debug: error writing prompt to codex stdin: %v", err)
		}
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
			ReasoningEffort:   reasoningEffort,
			CostUSD:           cp.computeCostUSD(usage, model),
			InputTokens:       usageInputTokens(usage),
			CachedInputTokens: usageCachedInputTokens(usage),
			OutputTokens:      usageOutputTokens(usage),
		}, nil
	}
	codexDebugf(output, "provider debug: cmd.Wait returned success")

	duration := time.Since(startTime)

	// If the turn ended with an error (e.g. UsageLimitExceeded), report failure
	if streamErrInfo != nil {
		failureCategory := classifyCodexFailure(0, resultText, "")
		if isCodexUsageLimitError(streamErrInfo) {
			failureCategory = FailureCategoryRateLimited
		}
		failureResult := &Result{
			Success:           false,
			Output:            resultText,
			Diagnostics:       buildCodexDiagnostics(args, effectiveCodexHome, ""),
			FailureCategory:   failureCategory,
			ExitCode:          0,
			Duration:          duration,
			Model:             model,
			ReasoningEffort:   reasoningEffort,
			CostUSD:           cp.computeCostUSD(usage, model),
			InputTokens:       usageInputTokens(usage),
			CachedInputTokens: usageCachedInputTokens(usage),
			OutputTokens:      usageOutputTokens(usage),
		}
		if isCodexUsageLimitError(streamErrInfo) {
			return failureResult, &UsageLimitError{
				Type:    streamErrInfo.Type,
				Message: streamErrInfo.Message,
			}
		}
		return failureResult, nil
	}

	return &Result{
		Success:           true,
		Output:            resultText,
		Diagnostics:       buildCodexDiagnostics(args, effectiveCodexHome, ""),
		ExitCode:          0,
		Duration:          duration,
		Model:             model,
		ReasoningEffort:   reasoningEffort,
		CostUSD:           cp.computeCostUSD(usage, model),
		InputTokens:       usageInputTokens(usage),
		CachedInputTokens: usageCachedInputTokens(usage),
		OutputTokens:      usageOutputTokens(usage),
	}, nil
}

func (cp *CodexProvider) runWithRetry(ctx context.Context, run func() (*Result, error)) (*Result, error) {
	var last *Result
	for attempt := 0; attempt <= codexTransientRetryMax; attempt++ {
		result, err := run()
		last = result
		if err != nil {
			if !shouldRetryCodexStartError(err, attempt) {
				return result, err
			}
			if sleepErr := cp.sleepFn(ctx, codexRetryBackoff(attempt)); sleepErr != nil {
				return result, err
			}
			continue
		}
		if result == nil || result.Success {
			return result, err
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
	cmd.WaitDelay = codexCommandWaitDelay
	cmd.Env = env
	reasoningEffort := reasoningEffortFromArgs(args)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	startTime := time.Now()

	if waitErr := procutil.WaitForProcessCapacity(ctx, codexProcessCapacityWait); waitErr != nil {
		return nil, fmt.Errorf("waiting for process capacity: %w", waitErr)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start codex command: %w", err)
	}
	codexKillDescendantsOnCancelFn(ctx, cmd)
	defer reapProcessGroupFn(cmd)

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
	output, usage := parseCodexOutputAndUsage(output)
	exitCode, err := cp.extractExitCode(err)
	if err != nil {
		return nil, err
	}

	return &Result{
		Success:           exitCode == 0,
		Output:            output,
		Stderr:            stderr.String(),
		Diagnostics:       buildCodexDiagnostics(args, effectiveCodexHome, stderr.String()),
		FailureCategory:   classifyCodexFailure(exitCode, output, stderr.String()),
		ExitCode:          exitCode,
		Duration:          duration,
		Model:             model,
		ReasoningEffort:   reasoningEffort,
		CostUSD:           cp.computeCostUSD(usage, model),
		InputTokens:       usageInputTokens(usage),
		CachedInputTokens: usageCachedInputTokens(usage),
		OutputTokens:      usageOutputTokens(usage),
	}, nil
}

// parseCodexOutputAndUsage normalizes JSONL output from non-streaming codex runs.
// It prefers structured stream parsing and falls back to legacy text extraction on parse failure.
func parseCodexOutputAndUsage(rawOutput string) (string, *codexUsage) {
	parsedText, usage, _, parseErr := processCodexStream(strings.NewReader(rawOutput), nil, nil, nil)
	if parseErr == nil {
		if parsedText != "" {
			return parsedText, usage
		}
		return rawOutput, usage
	}

	if text := extractAgentTextFromJSONL(rawOutput); text != "" {
		// Keep legacy text extraction as a fallback for unexpected parse failures.
		return text, usage
	}

	return rawOutput, usage
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

	if err != nil {
		var usageErr *UsageLimitError
		if errors.As(err, &usageErr) {
			return true
		}
	}

	// Must be a failure to be a usage limit error
	if result.Success {
		return false
	}
	return containsAnyKeywordCaseInsensitive(result.Output, usageLimitKeywords)
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
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%s", effort))
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

func reasoningEffortFromArgs(args []string) string {
	for i, arg := range args {
		if effort, ok := extractReasoningEffortValue(arg); ok {
			return effort
		}
		if (arg == "-c" || arg == "--config") && i+1 < len(args) {
			if effort, ok := extractReasoningEffortValue(args[i+1]); ok {
				return effort
			}
		}
	}
	return ""
}

func extractReasoningEffortValue(value string) (string, bool) {
	const key = "model_reasoning_effort="

	idx := strings.Index(value, key)
	if idx < 0 {
		return "", false
	}

	effort := value[idx+len(key):]
	if effort == "" {
		return "", false
	}

	if delim := strings.IndexAny(effort, ", \t\r\n\"'"); delim >= 0 {
		effort = effort[:delim]
	}
	effort = strings.TrimSpace(effort)
	if effort == "" {
		return "", false
	}

	return effort, true
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
