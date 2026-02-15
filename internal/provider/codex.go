package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

const (
	// providerNameCodex is the name identifier for the Codex provider
	providerNameCodex = "codex"
)

// CodexProvider wraps the Codex CLI and implements the Provider interface
type CodexProvider struct {
	binaryPath  string
	flags       []string
	tierToModel map[string]string
}

// Compile-time check to verify CodexProvider implements Provider interface
var _ Provider = (*CodexProvider)(nil)

// NewCodexProvider creates a new CodexProvider with the given configuration
func NewCodexProvider(binaryPath string, flags []string, tierToModel map[string]string) *CodexProvider {
	return &CodexProvider{
		binaryPath:  binaryPath,
		flags:       flags,
		tierToModel: tierToModel,
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

// Run executes an LLM invocation with the given prompt and tier
func (cp *CodexProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("codex provider is nil")
	}

	model := cp.ModelForTier(tier)
	args := cp.buildCommandArgs(model, false)
	cmd := exec.CommandContext(ctx, cp.binaryPath, args...)

	// Set up stdin pipe for prompt delivery
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start codex command: %w", err)
	}

	// Write prompt to stdin in a goroutine to avoid blocking.
	// Errors (e.g. broken pipe if process exits early) are non-fatal.
	go func() {
		defer stdin.Close()
		_, _ = io.WriteString(stdin, prompt)
	}()

	// Wait for command to complete
	err = cmd.Wait()
	duration := time.Since(startTime)

	if ctx.Err() != nil {
		return nil, fmt.Errorf("codex command cancelled: %w", ctx.Err())
	}

	output := stdout.String() + stderr.String()
	exitCode, err := cp.extractExitCode(err)
	if err != nil {
		return nil, err
	}

	return &Result{
		Success:  exitCode == 0,
		Output:   output,
		ExitCode: exitCode,
		Duration: duration,
		Model:    model,
	}, nil
}

// StreamRun executes an LLM invocation with streaming output.
// When EventHandler is non-nil, invokes codex with --json and parses JSONL events.
func (cp *CodexProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	if cp == nil {
		return nil, fmt.Errorf("codex provider is nil")
	}

	model := cp.ModelForTier(tier)
	args := cp.buildCommandArgs(model, handler != nil)
	cmd := exec.CommandContext(ctx, cp.binaryPath, args...)

	// Set up stdin pipe for prompt delivery
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	startTime := time.Now()

	// When handler is present, use processCodexStream to parse JSONL
	if handler != nil {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
		}

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start codex command: %w", err)
		}

		// Write prompt to stdin in goroutine
		go func() {
			defer stdin.Close()
			io.WriteString(stdin, prompt)
		}()

		// Process the JSONL stream
		resultText, usage, streamErrInfo, err := processCodexStream(stdout, output, handler, onToolCall)
		if err != nil {
			cmd.Wait()
			return nil, fmt.Errorf("failed to process codex stream: %w", err)
		}

		if err := cmd.Wait(); err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("codex command cancelled: %w", ctx.Err())
			}
			duration := time.Since(startTime)
			exitCode, _ := cp.extractExitCode(err)
			return &Result{
				Success:           false,
				Output:            resultText + stderr.String(),
				ExitCode:          exitCode,
				Duration:          duration,
				Model:             model,
				CostUSD:           usageCost(usage),
				InputTokens:       usageInputTokens(usage),
				CachedInputTokens: usageCachedInputTokens(usage),
				OutputTokens:      usageOutputTokens(usage),
			}, nil
		}

		duration := time.Since(startTime)

		// If the turn ended with an error (e.g. UsageLimitExceeded), report failure
		if streamErrInfo != nil {
			return &Result{
				Success:           false,
				Output:            resultText,
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
			ExitCode:          0,
			Duration:          duration,
			Model:             model,
			CostUSD:           usageCost(usage),
			InputTokens:       usageInputTokens(usage),
			CachedInputTokens: usageCachedInputTokens(usage),
			OutputTokens:      usageOutputTokens(usage),
		}, nil
	}

	// Without handler, use plain text capture
	var captureBuffer bytes.Buffer
	var multiWriter io.Writer
	if output != nil {
		multiWriter = io.MultiWriter(output, &captureBuffer)
	} else {
		multiWriter = &captureBuffer
	}

	var stderr bytes.Buffer
	cmd.Stdout = multiWriter
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start codex command: %w", err)
	}

	// Write prompt to stdin in goroutine
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, prompt)
	}()

	err = cmd.Wait()
	duration := time.Since(startTime)

	if ctx.Err() != nil {
		return nil, fmt.Errorf("codex command cancelled: %w", ctx.Err())
	}

	combinedOutput := captureBuffer.String() + stderr.String()
	exitCode, err := cp.extractExitCode(err)
	if err != nil {
		return nil, err
	}

	return &Result{
		Success:  exitCode == 0,
		Output:   combinedOutput,
		ExitCode: exitCode,
		Duration: duration,
		Model:    model,
	}, nil
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
	args := make([]string, 0, len(cp.flags)+12)
	args = append(args, "exec")
	args = append(args, cp.flags...)

	// --dangerously-bypass-approvals-and-sandbox and --full-auto are mutually
	// exclusive in the Codex CLI; only add --full-auto when the bypass flag
	// is not already present in user flags.
	hasBypass := false
	for _, f := range cp.flags {
		if f == "--dangerously-bypass-approvals-and-sandbox" {
			hasBypass = true
			break
		}
	}
	if !hasBypass {
		args = append(args, "--full-auto")
	}

	args = append(args, "--skip-git-repo-check")
	args = append(args, "--color", "never")
	args = append(args, "--model", model)
	if jsonMode {
		args = append(args, "--json")
	}
	args = append(args, "-")
	return args
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
	InputTokens       int `json:"input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	TotalCostUSD      float64 `json:"total_cost_usd,omitempty"`
}

// codexErrorInfo represents error information from Codex turn.completed events
type codexErrorInfo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
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
	var lastAgentText string
	var usage *codexUsage
	var errInfo *codexErrorInfo
	var sawDeltas bool

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event codexEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Skip malformed lines silently
			continue
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
				streamEvent := map[string]interface{}{
					"type":           "result",
					"total_cost_usd": event.TotalCostUSD,
					"input_tokens":   event.Usage.InputTokens,
					"output_tokens":  event.Usage.OutputTokens,
				}
				eventJSON, _ := json.Marshal(streamEvent)
				handler(eventJSON)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", nil, nil, err
	}

	return lastAgentText, usage, errInfo, nil
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
