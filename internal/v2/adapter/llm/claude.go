package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/procutil"
)

const claudeProcessCapacityWait = 1500 * time.Millisecond

// NewClaudeAdapter returns an LLMProvider backed by the Claude CLI.
func NewClaudeAdapter(binary string, flags []string, timeout time.Duration) LLMProvider {
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	copiedFlags := append([]string(nil), flags...)
	return &claudeAdapter{
		binary:  binary,
		flags:   copiedFlags,
		timeout: timeout,
	}
}

type claudeAdapter struct {
	binary  string
	flags   []string
	timeout time.Duration
}

func (a *claudeAdapter) Invoke(ctx context.Context, req InvokeRequest) (*LLMResponse, error) {
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	args := []string{"-p", "--model", req.Model, "--output-format", "json"}
	args = append(args, a.flags...)

	stdout, duration, err := a.runOnce(ctx, args, req.Prompt, strings.TrimSpace(req.Dir))
	if err != nil {
		return nil, err
	}

	parsed, err := parseInvokeResult(stdout)
	if err != nil {
		return nil, fmt.Errorf("parse claude json: %w", err)
	}

	return &LLMResponse{
		Success:  parsed.Success,
		Output:   parsed.Output,
		Tokens:   parsed.Tokens,
		CostUSD:  parsed.CostUSD,
		Duration: duration,
	}, nil
}

func (a *claudeAdapter) StreamInvoke(ctx context.Context, req StreamInvokeRequest) (*LLMResponse, error) {
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	output := req.Output
	if output == nil {
		output = io.Discard
	}

	flags := append([]string(nil), a.flags...)
	timeoutSec := int(a.timeout / time.Second)
	if timeoutSec <= 0 {
		timeoutSec = 1
	}

	client, err := claude.NewClient(a.binary, flags, timeoutSec)
	if err != nil {
		return nil, err
	}

	var runOpts []claude.RunOption
	if dir := strings.TrimSpace(req.Dir); dir != "" {
		runOpts = append(runOpts, claude.WithDir(dir))
	}

	result, err := client.StreamRun(ctx, req.Prompt, req.Model, output, nil, nil, runOpts...)
	if err != nil {
		return nil, err
	}

	return &LLMResponse{
		Success:  result.Success,
		Output:   strings.TrimSpace(result.Output),
		Tokens:   result.InputTokens + result.OutputTokens,
		CostUSD:  result.CostUSD,
		Duration: result.Duration,
	}, nil
}

func (a *claudeAdapter) runOnce(ctx context.Context, args []string, prompt string, dir string) (string, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	if err := procutil.WaitForProcessCapacity(ctx, claudeProcessCapacityWait); err != nil {
		return "", 0, fmt.Errorf("running claude: waiting for process capacity: %w", err)
	}

	cmd := exec.CommandContext(ctx, a.binary, args...)
	procutil.SetProcessGroupKill(cmd)
	cmd.Env = procutil.SubprocessEnv()
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", 0, fmt.Errorf("creating stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", 0, fmt.Errorf("starting claude: %w", err)
	}
	defer procutil.ReapProcessTree(cmd)

	go func() {
		defer stdin.Close()
		if _, err := io.WriteString(stdin, prompt); err != nil {
			_ = err // best-effort; nothing to do
		}
	}()

	start := time.Now()
	err = cmd.Wait()
	duration := time.Since(start)

	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", 0, fmt.Errorf("running claude: %w", ctxErr)
		}
		if _, ok := err.(*exec.ExitError); !ok {
			return "", 0, fmt.Errorf("running claude: %w", err)
		}
		// Include stderr in output for debugging failed invocations.
		if stderr.Len() > 0 {
			stdout.WriteString("\n\nSTDERR:\n")
			stdout.WriteString(stderr.String())
		}
	}

	return stdout.String(), duration, nil
}

func parseInvokeResult(raw string) (*LLMResponse, error) {
	candidate := extractJSON(raw)
	if candidate == "" {
		return nil, errors.New("no json output")
	}

	var payload struct {
		Success bool   `json:"success"`
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
		Output  string `json:"output"`
		Message *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
		Usage struct {
			TotalCostUSD float64 `json:"total_cost_usd"`
			InputTokens  int     `json:"input_tokens"`
			OutputTokens int     `json:"output_tokens"`
		} `json:"usage"`
		TotalCostUSD float64 `json:"total_cost_usd"`
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
	}

	if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(payload.Result)
	if text == "" {
		text = strings.TrimSpace(payload.Output)
	}
	if text == "" && payload.Message != nil {
		var builder strings.Builder
		for _, block := range payload.Message.Content {
			if block.Text != "" {
				builder.WriteString(block.Text)
			}
		}
		text = strings.TrimSpace(builder.String())
	}

	cost := payload.TotalCostUSD
	if cost == 0 {
		cost = payload.Usage.TotalCostUSD
	}
	inputTokens := payload.InputTokens
	if inputTokens == 0 {
		inputTokens = payload.Usage.InputTokens
	}
	outputTokens := payload.OutputTokens
	if outputTokens == 0 {
		outputTokens = payload.Usage.OutputTokens
	}

	// Claude CLI uses "is_error" (not "success") in --output-format json.
	// When "success" is absent it defaults to false; fall back to !is_error.
	success := payload.Success
	if !success {
		success = !payload.IsError
	}

	return &LLMResponse{
		Success: success,
		Output:  text,
		Tokens:  inputTokens + outputTokens,
		CostUSD: cost,
	}, nil
}

func extractJSON(output string) string {
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start == -1 || end == -1 || end <= start {
		return strings.TrimSpace(output)
	}
	return strings.TrimSpace(output[start : end+1])
}
