package llm

import (
	"bufio"
	"context"
	"encoding/json"
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

const codexProcessCapacityWait = 1500 * time.Millisecond

// codexTransientRetry constants for bounded retry on transport/rate-limit errors.
const (
	codexTransientRetryMax   = 2
	codexRetryBackoffFirst   = 250 * time.Millisecond
	codexRetryBackoffSecond  = 750 * time.Millisecond
	codexRetryBackoffDefault = 1500 * time.Millisecond
)

var _ LLMProvider = (*codexAdapter)(nil)

// CodexOption configures a codexAdapter.
type CodexOption func(*codexAdapter)

// WithReasoningEffort sets per-tier reasoning effort values (e.g. {"high":"high"}).
func WithReasoningEffort(tierToEffort map[string]string) CodexOption {
	return func(a *codexAdapter) {
		a.tierToReasoning = make(map[string]string, len(tierToEffort))
		for tier, effort := range tierToEffort {
			key := strings.ToLower(strings.TrimSpace(tier))
			val := strings.ToLower(strings.TrimSpace(effort))
			if key != "" && val != "" {
				a.tierToReasoning[key] = val
			}
		}
	}
}

// NewCodexAdapter returns an LLMProvider backed by the Codex CLI.
func NewCodexAdapter(binary string, flags []string, timeout time.Duration, opts ...CodexOption) LLMProvider {
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	copiedFlags := append([]string(nil), flags...)
	a := &codexAdapter{
		binary:          binary,
		flags:           copiedFlags,
		timeout:         timeout,
		tierToReasoning: map[string]string{},
		sleepFn:         sleepWithContext,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type codexAdapter struct {
	binary          string
	flags           []string
	timeout         time.Duration
	tierToReasoning map[string]string
	sleepFn         func(context.Context, time.Duration) error
}

func (a *codexAdapter) Invoke(ctx context.Context, req InvokeRequest) (*LLMResponse, error) {
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	tier := req.Metadata["tier"]
	args := a.buildExecCommandArgs(req.Model, tier)
	env := a.prepareEnv()
	dir := strings.TrimSpace(req.Dir)

	resp, err := a.runWithRetry(ctx, func() (*LLMResponse, error) {
		return a.runOnce(ctx, args, req.Prompt, dir, env)
	})
	return resp, err
}

func (a *codexAdapter) StreamInvoke(ctx context.Context, req StreamInvokeRequest) (*LLMResponse, error) {
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	output := req.Output
	if output == nil {
		output = io.Discard
	}

	tier := req.Metadata["tier"]
	args := a.buildExecCommandArgs(req.Model, tier)
	env := a.prepareEnv()
	dir := strings.TrimSpace(req.Dir)

	resp, err := a.runWithRetry(ctx, func() (*LLMResponse, error) {
		return a.streamRunOnce(ctx, args, req.Prompt, dir, env, output)
	})
	return resp, err
}

// buildExecCommandArgs constructs codex CLI args:
// exec, flags..., [--full-auto], --skip-git-repo-check, --color, never, --model, model, [-c model_reasoning_effort=X], --json, -
func (a *codexAdapter) buildExecCommandArgs(model, tier string) []string {
	args := make([]string, 0, len(a.flags)+12)
	args = append(args, "exec")
	args = append(args, a.flags...)

	if !hasBypassFlag(a.flags) {
		args = append(args, "--full-auto")
	}

	args = append(args, "--skip-git-repo-check")
	args = append(args, "--color", "never")
	args = append(args, "--model", model)

	if effort, ok := a.reasoningEffortForTier(tier); ok && !hasReasoningEffortInFlags(a.flags) {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%s", effort))
	}

	args = append(args, "--json")
	args = append(args, "-")
	return args
}

func (a *codexAdapter) reasoningEffortForTier(tier string) (string, bool) {
	if len(a.tierToReasoning) == 0 {
		return "", false
	}
	effort, ok := a.tierToReasoning[strings.ToLower(strings.TrimSpace(tier))]
	if !ok || strings.TrimSpace(effort) == "" {
		return "", false
	}
	return effort, true
}

func hasBypassFlag(flags []string) bool {
	for _, f := range flags {
		if f == "--dangerously-bypass-approvals-and-sandbox" {
			return true
		}
	}
	return false
}

func hasReasoningEffortInFlags(flags []string) bool {
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

func (a *codexAdapter) prepareEnv() []string {
	env := procutil.SubprocessEnv()

	// Resolve CODEX_HOME to a safe location if set.
	configuredHome, ok := os.LookupEnv("CODEX_HOME")
	if !ok {
		return env
	}
	codexHome := strings.TrimSpace(configuredHome)
	if codexHome == "" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			codexHome = filepath.Join(home, ".codex")
		}
	}
	if codexHome != "" {
		env = upsertEnvVar(env, "CODEX_HOME", codexHome)
	}
	return env
}

func upsertEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func (a *codexAdapter) runOnce(ctx context.Context, args []string, prompt, dir string, env []string) (*LLMResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	if err := procutil.WaitForProcessCapacity(ctx, codexProcessCapacityWait); err != nil {
		return nil, fmt.Errorf("running codex: waiting for process capacity: %w", err)
	}

	cmd := exec.CommandContext(ctx, a.binary, args...)
	procutil.SetProcessGroupKill(cmd)
	cmd.Env = env
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting codex: %w", err)
	}
	defer procutil.ReapProcessTree(cmd)

	go func() {
		defer stdin.Close()
		if _, err := io.WriteString(stdin, prompt); err != nil {
			_ = err
		}
	}()

	start := time.Now()
	err = cmd.Wait()
	duration := time.Since(start)

	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("running codex: %w", ctxErr)
		}
		exitErr, isExit := err.(*exec.ExitError)
		if !isExit {
			return nil, fmt.Errorf("running codex: %w", err)
		}
		_ = exitErr // non-zero exit: return failure with stderr captured.
		return &LLMResponse{
			Success:  false,
			Output:   strings.TrimSpace(stderr.String()),
			Duration: duration,
		}, nil
	}

	// Parse JSONL output.
	text, usage := parseCodexJSONLOutput(stdout.String())

	return &LLMResponse{
		Success:      true,
		Output:       text,
		Tokens:       codexUsageTokens(usage),
		InputTokens:  codexInputTokens(usage),
		OutputTokens: codexOutputTokens(usage),
		CostUSD:      codexUsageCost(usage),
		Duration:     duration,
	}, nil
}

func (a *codexAdapter) streamRunOnce(ctx context.Context, args []string, prompt, dir string, env []string, output io.Writer) (*LLMResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	if err := procutil.WaitForProcessCapacity(ctx, codexProcessCapacityWait); err != nil {
		return nil, fmt.Errorf("running codex: waiting for process capacity: %w", err)
	}

	cmd := exec.CommandContext(ctx, a.binary, args...)
	procutil.SetProcessGroupKill(cmd)
	cmd.Env = env
	if dir != "" {
		cmd.Dir = dir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	var stderr strings.Builder
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting codex: %w", err)
	}
	defer procutil.ReapProcessTree(cmd)

	go func() {
		defer stdin.Close()
		if _, err := io.WriteString(stdin, prompt); err != nil {
			_ = err
		}
	}()

	start := time.Now()

	// Read and process JSONL stream, writing agent text to output.
	text, usage := processCodexJSONLStream(stdout, output)

	err = cmd.Wait()
	duration := time.Since(start)

	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("running codex: %w", ctxErr)
		}
		if _, isExit := err.(*exec.ExitError); !isExit {
			return nil, fmt.Errorf("running codex: %w", err)
		}
		return &LLMResponse{
			Success:  false,
			Output:   strings.TrimSpace(stderr.String()),
			Duration: duration,
		}, nil
	}

	return &LLMResponse{
		Success:      true,
		Output:       text,
		Tokens:       codexUsageTokens(usage),
		InputTokens:  codexInputTokens(usage),
		OutputTokens: codexOutputTokens(usage),
		CostUSD:      codexUsageCost(usage),
		Duration:     duration,
	}, nil
}

// runWithRetry wraps a run function with bounded transient retry.
func (a *codexAdapter) runWithRetry(ctx context.Context, run func() (*LLMResponse, error)) (*LLMResponse, error) {
	var last *LLMResponse
	for attempt := 0; attempt <= codexTransientRetryMax; attempt++ {
		result, err := run()
		last = result
		if err != nil {
			if !isTransientStartError(err) || attempt >= codexTransientRetryMax {
				return result, err
			}
			if sleepErr := a.sleepFn(ctx, codexRetryBackoff(attempt)); sleepErr != nil {
				return result, err
			}
			continue
		}
		if result == nil || result.Success {
			return result, nil
		}
		if !isTransientOutput(result.Output) || attempt >= codexTransientRetryMax {
			return result, nil
		}
		if sleepErr := a.sleepFn(ctx, codexRetryBackoff(attempt)); sleepErr != nil {
			return result, nil
		}
	}
	return last, nil
}

func isTransientStartError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "resource temporarily unavailable")
}

func isTransientOutput(output string) bool {
	lower := strings.ToLower(output)
	for _, pattern := range []string{
		"transport_disconnect", "stream disconnected", "connection reset",
		"rate limit", "too many requests", "429",
	} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
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
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// --- JSONL parsing ---

// codexJSONLUsage tracks token usage from codex JSONL events.
type codexJSONLUsage struct {
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`
}

func codexUsageTokens(u *codexJSONLUsage) int {
	if u == nil {
		return 0
	}
	return u.InputTokens + u.OutputTokens
}

func codexInputTokens(u *codexJSONLUsage) int {
	if u == nil {
		return 0
	}
	return u.InputTokens
}

func codexOutputTokens(u *codexJSONLUsage) int {
	if u == nil {
		return 0
	}
	return u.OutputTokens
}

func codexUsageCost(u *codexJSONLUsage) float64 {
	if u == nil {
		return 0
	}
	return u.TotalCostUSD
}

// parseCodexJSONLOutput parses JSONL output from a non-streaming codex run.
// Returns the last agent_message text and accumulated usage.
func parseCodexJSONLOutput(raw string) (string, *codexJSONLUsage) {
	var lastText string
	var usage codexJSONLUsage
	var foundUsage bool

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event codexJSONLEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if text := extractCodexAgentText(event); text != "" {
			lastText = text
		}
		if u := extractCodexUsage(event); u != nil {
			usage.InputTokens = u.InputTokens
			usage.OutputTokens = u.OutputTokens
			usage.TotalCostUSD = u.TotalCostUSD
			foundUsage = true
		}
	}

	if lastText == "" {
		lastText = raw
	}

	if foundUsage {
		return lastText, &usage
	}
	return lastText, nil
}

// processCodexJSONLStream reads JSONL events from reader, writes agent text to output,
// and returns final text + usage.
func processCodexJSONLStream(reader io.Reader, output io.Writer) (string, *codexJSONLUsage) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var lastText string
	var usage codexJSONLUsage
	var foundUsage bool

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event codexJSONLEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		// Stream incremental deltas to output.
		if event.Type == "item.agentMessage.delta" && event.Delta != nil && event.Delta.Text != "" {
			if output != nil {
				_, _ = output.Write([]byte(event.Delta.Text))
			}
		}

		if text := extractCodexAgentText(event); text != "" {
			lastText = text
			// Write completed agent message if no deltas were streamed.
			if output != nil && event.Type == "item.completed" {
				_, _ = output.Write([]byte(text))
			}
		}

		if u := extractCodexUsage(event); u != nil {
			usage.InputTokens = u.InputTokens
			usage.OutputTokens = u.OutputTokens
			usage.TotalCostUSD = u.TotalCostUSD
			foundUsage = true
		}
	}

	if foundUsage {
		return lastText, &usage
	}
	return lastText, nil
}

// codexJSONLEvent is a minimal representation of codex JSONL events.
type codexJSONLEvent struct {
	Type         string              `json:"type"`
	Text         string              `json:"text,omitempty"`
	Item         *codexJSONLItem     `json:"item,omitempty"`
	Delta        *codexJSONLDelta    `json:"delta,omitempty"`
	Usage        *codexJSONLUsage    `json:"usage,omitempty"`
	Result       *codexJSONLResult   `json:"result,omitempty"`
	Response     *codexJSONLResponse `json:"response,omitempty"`
	InputTokens  int                 `json:"input_tokens,omitempty"`
	OutputTokens int                 `json:"output_tokens,omitempty"`
	TotalCostUSD float64             `json:"total_cost_usd,omitempty"`
}

type codexJSONLItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type codexJSONLDelta struct {
	Text string `json:"text"`
}

type codexJSONLResult struct {
	Usage        *codexJSONLUsage `json:"usage,omitempty"`
	InputTokens  int              `json:"input_tokens,omitempty"`
	OutputTokens int              `json:"output_tokens,omitempty"`
	TotalCostUSD float64          `json:"total_cost_usd,omitempty"`
}

type codexJSONLResponse struct {
	Usage        *codexJSONLUsage `json:"usage,omitempty"`
	InputTokens  int              `json:"input_tokens,omitempty"`
	OutputTokens int              `json:"output_tokens,omitempty"`
	TotalCostUSD float64          `json:"total_cost_usd,omitempty"`
}

func extractCodexAgentText(event codexJSONLEvent) string {
	if event.Type == "item.completed" && event.Item != nil && event.Item.Type == "agent_message" {
		return event.Item.Text
	}
	return ""
}

func extractCodexUsage(event codexJSONLEvent) *codexJSONLUsage {
	switch event.Type {
	case "turn.completed", "result", "response.completed":
	default:
		return nil
	}

	var u codexJSONLUsage
	found := false

	if event.Usage != nil {
		u = *event.Usage
		found = true
	}
	if event.InputTokens > 0 {
		u.InputTokens = event.InputTokens
		found = true
	}
	if event.OutputTokens > 0 {
		u.OutputTokens = event.OutputTokens
		found = true
	}
	if event.TotalCostUSD > 0 {
		u.TotalCostUSD = event.TotalCostUSD
		found = true
	}

	// Check nested result/response usage.
	if event.Result != nil {
		if event.Result.Usage != nil {
			if event.Result.Usage.InputTokens > 0 {
				u.InputTokens = event.Result.Usage.InputTokens
				found = true
			}
			if event.Result.Usage.OutputTokens > 0 {
				u.OutputTokens = event.Result.Usage.OutputTokens
				found = true
			}
			if event.Result.Usage.TotalCostUSD > 0 {
				u.TotalCostUSD = event.Result.Usage.TotalCostUSD
				found = true
			}
		}
		if event.Result.InputTokens > 0 {
			u.InputTokens = event.Result.InputTokens
			found = true
		}
		if event.Result.OutputTokens > 0 {
			u.OutputTokens = event.Result.OutputTokens
			found = true
		}
		if event.Result.TotalCostUSD > 0 {
			u.TotalCostUSD = event.Result.TotalCostUSD
			found = true
		}
	}
	if event.Response != nil {
		if event.Response.Usage != nil {
			if event.Response.Usage.InputTokens > 0 {
				u.InputTokens = event.Response.Usage.InputTokens
				found = true
			}
			if event.Response.Usage.OutputTokens > 0 {
				u.OutputTokens = event.Response.Usage.OutputTokens
				found = true
			}
			if event.Response.Usage.TotalCostUSD > 0 {
				u.TotalCostUSD = event.Response.Usage.TotalCostUSD
				found = true
			}
		}
		if event.Response.InputTokens > 0 {
			u.InputTokens = event.Response.InputTokens
			found = true
		}
		if event.Response.OutputTokens > 0 {
			u.OutputTokens = event.Response.OutputTokens
			found = true
		}
		if event.Response.TotalCostUSD > 0 {
			u.TotalCostUSD = event.Response.TotalCostUSD
			found = true
		}
	}

	if !found {
		return nil
	}
	return &u
}
