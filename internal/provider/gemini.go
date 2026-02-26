package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

const (
	providerNameGemini            = "gemini"
	geminiShortPromptThreshold    = 256 // Use -p flag for prompts under this size
)


// GeminiProvider wraps the Gemini CLI and implements the Provider interface
// for JSON and streaming invocations.
type GeminiProvider struct {
	binary        string
	flags         []string
	tierToModel   map[string]string
	runFn         geminiRunFn
	costEstimator geminiCostEstimator
}

// Compile-time check to verify GeminiProvider satisfies Provider.
var _ Provider = (*GeminiProvider)(nil)

type geminiRunResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	duration time.Duration
}

type geminiRunFn func(ctx context.Context, binary string, args []string, prompt string) (*geminiRunResult, error)

// NewGeminiProvider constructs a GeminiProvider with the specified binary, flags,
// and tier-to-model mapping.
func NewGeminiProvider(binary string, flags []string, tierToModel map[string]string, estimator geminiCostEstimator) *GeminiProvider {
	if tierToModel == nil {
		tierToModel = map[string]string{}
	}
	return &GeminiProvider{
		binary:        binary,
		flags:         append([]string(nil), flags...),
		tierToModel:   tierToModel,
		runFn:         defaultGeminiRunFn,
		costEstimator: estimator,
	}
}

// Name returns the provider identifier.
func (gp *GeminiProvider) Name() string {
	return providerNameGemini
}

// ModelForTier maps abstract tiers to concrete Gemini models.
func (gp *GeminiProvider) ModelForTier(tier string) string {
	if model, ok := gp.tierToModel[tier]; ok {
		return model
	}
	return ""
}

// Run executes a non-streaming Gemini invocation and parses the JSON result.
// Uses stdin-first delivery for large prompts, falls back to -p flag for short prompts.
func (gp *GeminiProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	if gp == nil {
		return nil, fmt.Errorf("gemini provider is nil")
	}

	model := gp.ModelForTier(tier)

	// Decide delivery method based on prompt size
	var args []string
	if len(prompt) <= geminiShortPromptThreshold {
		// Use inline -p flag for short prompts (fallback)
		args = gp.buildCommandArgs(model, "json", prompt)
	} else {
		// Use stdin for large prompts (primary delivery method)
		args = gp.buildCommandArgsForStdin(model, "json")
	}

	runner := gp.runFn
	if runner == nil {
		runner = defaultGeminiRunFn
	}

	execResult, err := runner(ctx, gp.binary, args, prompt)
	if err != nil {
		return nil, err
	}
	if execResult == nil {
		return nil, fmt.Errorf("gemini run returned nil result")
	}

	result, err := buildGeminiResult(execResult, model, gp.costEstimator)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// StreamRun is not implemented yet and returns an error placeholder.
// When implemented, should use stdin-first prompt delivery like Run().
func (gp *GeminiProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
	return nil, ErrStreamNotSupported
}

func (gp *GeminiProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
	if gp == nil {
		return nil, fmt.Errorf("gemini provider is nil")
	}

	// Validate commands
	if err := ValidateCommands(commands); err != nil {
		return nil, err
	}

	// Build validation prompt
	prompt := BuildValidationPrompt(commands, workDir)

	// Run the validation prompt with explicit working directory
	return gp.runWithWorkDir(ctx, prompt, tier, workDir)
}

func (gp *GeminiProvider) runWithWorkDir(ctx context.Context, prompt string, tier string, workDir string) (*Result, error) {
	if gp == nil {
		return nil, fmt.Errorf("gemini provider is nil")
	}

	model := gp.ModelForTier(tier)
	args := gp.buildCommandArgsForStdin(model, "json")

	runner := gp.runFn
	if runner == nil {
		runner = defaultGeminiRunFn
	}

	execResult, err := runner(ctx, gp.binary, args, prompt)
	if err != nil {
		return nil, err
	}
	if execResult == nil {
		return nil, fmt.Errorf("gemini run returned nil result")
	}

	result, err := buildGeminiResult(execResult, model, gp.costEstimator)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (gp *GeminiProvider) IsUsageLimitError(result *Result, err error) bool {
	if result == nil {
		return false
	}
	// Must have exit code 2 to be a usage limit error
	if result.ExitCode != 2 {
		return false
	}
	return containsAnyKeywordCaseInsensitive(result.Output, usageLimitKeywords)
}

func (gp *GeminiProvider) IsValidationPassed(result *Result) bool {
	return IsValidationPassed(result)
}

func (gp *GeminiProvider) IsScopeTooLarge(result *Result) (bool, string) {
	return IsScopeTooLarge(result)
}

func (gp *GeminiProvider) buildCommandArgs(model, outputFormat, prompt string) []string {
	args := make([]string, 0, len(gp.flags)+7)
	args = append(args, gp.flags...)
	if outputFormat != "" {
		args = append(args, "--output-format", outputFormat)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "-p", prompt)
	return args
}

func (gp *GeminiProvider) buildCommandArgsForStdin(model, outputFormat string) []string {
	args := make([]string, 0, len(gp.flags)+6)
	args = append(args, gp.flags...)
	if outputFormat != "" {
		args = append(args, "--output-format", outputFormat)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "-")
	return args
}

func buildGeminiResult(execResult *geminiRunResult, defaultModel string, estimator geminiCostEstimator) (*Result, error) {
	payload, err := extractJSONPayload(execResult.stdout)
	if err != nil {
		return buildGeminiFallbackResult(execResult, defaultModel, err)
	}

	parsed, err := parseGeminiJSONResult(payload, defaultModel, estimator)
	if err != nil {
		return buildGeminiFallbackResult(execResult, defaultModel, err)
	}

	parsed.ExitCode = execResult.exitCode
	parsed.Stderr = string(execResult.stderr)
	parsed.Duration = execResult.duration
	parsed.Success = execResult.exitCode == 0
	parsed.FailureCategory = classifyGeminiFailure(execResult.exitCode, parsed.Stderr)

	if parsed.Model == "" {
		parsed.Model = defaultModel
	}

	return parsed, nil
}

func buildGeminiFallbackResult(execResult *geminiRunResult, defaultModel string, parseErr error) (*Result, error) {
	classification := classifyGeminiCLIError(string(execResult.stderr))
	diagnostics := fmt.Sprintf("gemini_error_category=%s retryable=%t exit_code=%d", classification.Category, classification.Retryable, execResult.exitCode)
	if parseErr != nil {
		diagnostics = fmt.Sprintf("%s json_error=%q", diagnostics, parseErr.Error())
	}

	return &Result{
		Success:         execResult.exitCode == 0,
		Output:          string(execResult.stdout),
		Stderr:          string(execResult.stderr),
		Diagnostics:     diagnostics,
		FailureCategory: classifyGeminiFailure(execResult.exitCode, string(execResult.stderr)),
		ExitCode:        execResult.exitCode,
		Duration:        execResult.duration,
		Model:           defaultModel,
	}, nil
}

func extractJSONPayload(data []byte) ([]byte, error) {
	depth := 0
	start := -1
	inString := false
	escape := false

	for i := 0; i < len(data); i++ {
		b := data[i]
		if escape {
			escape = false
			continue
		}
		switch b {
		case '\\':
			escape = true
			continue
		case '"':
			inString = !inString
		}

		if inString {
			continue
		}

		switch b {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					return data[start : i+1], nil
				}
			}
		}
	}

	if start >= 0 {
		return nil, fmt.Errorf("incomplete gemini json payload")
	}
	return nil, fmt.Errorf("gemini json payload not found")
}

func defaultGeminiRunFn(ctx context.Context, binary string, args []string, prompt string) (*geminiRunResult, error) {
	cmd := execCommandContext(ctx, binary, args...)

	// Check if this is a stdin invocation (last arg is "-")
	// For inline -p invocations, the prompt is already in the args, no stdin needed
	isStdinInvocation := len(args) > 0 && args[len(args)-1] == "-"

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()

	if isStdinInvocation {
		// Use stdin pipe for prompt delivery
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start gemini command: %w", err)
		}

		// Write prompt to stdin in goroutine
		go func() {
			defer stdin.Close()
			io.WriteString(stdin, prompt)
		}()

		err = cmd.Wait()
		duration := time.Since(start)

		exitCode := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				return nil, fmt.Errorf("failed to execute gemini command: %w", err)
			}
		}

		return &geminiRunResult{
			stdout:   stdout.Bytes(),
			stderr:   stderr.Bytes(),
			exitCode: exitCode,
			duration: duration,
		}, nil
	}

	// Inline -p flag path: prompt is already in args, no stdin needed
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute gemini command: %w", err)
		}
	}

	return &geminiRunResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
		duration: duration,
	}, nil
}
