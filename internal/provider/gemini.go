package provider

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os/exec"
    "time"
)

const providerNameGemini = "gemini"

// GeminiProvider wraps the Gemini CLI and implements the Provider interface
// for JSON and streaming invocations.
type GeminiProvider struct {
	binary      string
	flags       []string
	tierToModel map[string]string
	runFn       geminiRunFn
}

// Compile-time check to verify GeminiProvider satisfies Provider.
var _ Provider = (*GeminiProvider)(nil)

type geminiRunResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	duration time.Duration
}

type geminiRunFn func(ctx context.Context, binary string, args []string) (*geminiRunResult, error)

// NewGeminiProvider constructs a GeminiProvider with the specified binary, flags,
// and tier-to-model mapping.
func NewGeminiProvider(binary string, flags []string, tierToModel map[string]string) *GeminiProvider {
	if tierToModel == nil {
		tierToModel = map[string]string{}
	}
	return &GeminiProvider{
		binary:      binary,
		flags:       append([]string(nil), flags...),
		tierToModel: tierToModel,
		runFn:       defaultGeminiRunFn,
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
	return tier
}

// Run executes a non-streaming Gemini invocation and parses the JSON result.
func (gp *GeminiProvider) Run(ctx context.Context, prompt string, tier string) (*Result, error) {
	if gp == nil {
		return nil, fmt.Errorf("gemini provider is nil")
	}

	model := gp.ModelForTier(tier)
	args := gp.buildCommandArgs(model, "json", prompt)

	runner := gp.runFn
	if runner == nil {
		runner = defaultGeminiRunFn
	}

	execResult, err := runner(ctx, gp.binary, args)
	if err != nil {
		return nil, err
	}
	if execResult == nil {
		return nil, fmt.Errorf("gemini run returned nil result")
	}

	result, err := buildGeminiResult(execResult, model)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// StreamRun is not implemented yet and returns an error placeholder.
func (gp *GeminiProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
    handler EventHandler, onToolCall ToolCallHandler) (*Result, error) {
    return nil, fmt.Errorf("stream run not implemented")
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

	// Run the validation prompt
	return gp.Run(ctx, prompt, tier)
}

func (gp *GeminiProvider) IsUsageLimitError(result *Result, err error) bool {
    return false
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

func buildGeminiResult(execResult *geminiRunResult, defaultModel string) (*Result, error) {
	payload, err := extractJSONPayload(execResult.stdout)
	if err != nil {
		return nil, err
	}

	parsed, err := parseGeminiJSONResult(payload)
	if err != nil {
		return nil, err
	}

	parsed.ExitCode = execResult.exitCode
	parsed.Stderr = string(execResult.stderr)
	parsed.Duration = execResult.duration
	parsed.Success = execResult.exitCode == 0
	parsed.FailureCategory = classifyGeminiFailure(execResult.exitCode, parsed.Stderr)

	if parsed.Model == "" {
		parsed.Model = defaultModel
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(payload, &jsonData); err == nil {
		parsed.CostUSD = extractGeminiCost(jsonData)
	}

	return parsed, nil
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

func defaultGeminiRunFn(ctx context.Context, binary string, args []string) (*geminiRunResult, error) {
	cmd := execCommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
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
