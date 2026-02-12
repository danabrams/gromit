package agent

import (
	"fmt"
	"os"
	"os/exec"
)

// PromptDelivery defines how an agent receives its prompt
type PromptDelivery string

const (
	// FileRef passes a message to the agent telling it to read a temp file
	FileRef PromptDelivery = "file_ref"
	// PromptFileArg passes the temp file path as a flag argument
	PromptFileArg PromptDelivery = "prompt_file_arg"
	// Stdin pipes the prompt content to the agent's stdin
	Stdin PromptDelivery = "stdin"
)

const fileRefMessageFormat = "Read and follow instructions in %s"

// Agent represents an AI agent that can be launched with a prompt
type Agent interface {
	// Name returns the agent's name
	Name() string
	// Launch executes the agent with the given prompt file path
	Launch(promptPath string) error
	// Command builds a configured *exec.Cmd for the agent without starting it
	Command(promptPath string) (*exec.Cmd, error)
}

// cliAgent is a CLI-based agent implementation
type cliAgent struct {
	name           string
	binary         string
	flags          []string
	promptDelivery PromptDelivery
	promptFlag     string
	extraArgs      []string
}

// New creates a new CLI agent with the given configuration
func New(name, binary string, flags []string, promptDelivery PromptDelivery, promptFlag string, extraArgs []string) Agent {
	return &cliAgent{
		name:           name,
		binary:         binary,
		flags:          flags,
		promptDelivery: promptDelivery,
		promptFlag:     promptFlag,
		extraArgs:      extraArgs,
	}
}

// Name returns the agent's name
func (a *cliAgent) Name() string {
	return a.name
}

// Launch executes the agent with the given prompt file path
func (a *cliAgent) Launch(promptPath string) error {
	// Verify prompt file exists
	if _, err := os.Stat(promptPath); err != nil {
		return fmt.Errorf("prompt file not found: %w", err)
	}

	// Build command arguments based on prompt delivery method
	args := make([]string, 0, len(a.flags)+len(a.extraArgs)+3)
	args = append(args, a.flags...)

	switch a.promptDelivery {
	case FileRef:
		// Add the file reference message as a positional argument
		args = append(args, fmt.Sprintf(fileRefMessageFormat, promptPath))
	case PromptFileArg:
		// Add the prompt flag and file path
		if a.promptFlag != "" {
			args = append(args, a.promptFlag, promptPath)
		}
	case Stdin:
		// No additional args needed - prompt will be piped to stdin
	}

	args = append(args, a.extraArgs...)

	// Create command
	cmd := exec.Command(a.binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// For Stdin delivery, read prompt file and pipe to stdin
	if a.promptDelivery == Stdin {
		content, err := os.ReadFile(promptPath)
		if err != nil {
			return fmt.Errorf("failed to read prompt file: %w", err)
		}
		r, w, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}
		cmd.Stdin = r
		go func() {
			defer w.Close()
			_, _ = w.Write(content) // Best-effort write; command will fail if it can't read
		}()
	} else {
		cmd.Stdin = os.Stdin
	}

	// Run the command
	err := cmd.Run()

	// Treat exec.ExitError as graceful exit (agent returned non-zero)
	if _, ok := err.(*exec.ExitError); ok {
		return nil
	}

	return err
}

// Command builds a configured *exec.Cmd for the agent without starting it
func (a *cliAgent) Command(promptPath string) (*exec.Cmd, error) {
	// Verify prompt file exists
	if _, err := os.Stat(promptPath); err != nil {
		return nil, fmt.Errorf("prompt file not found: %w", err)
	}

	// Build command arguments based on prompt delivery method
	args := make([]string, 0, len(a.flags)+len(a.extraArgs)+3)
	args = append(args, a.flags...)

	switch a.promptDelivery {
	case FileRef:
		// Add the file reference message as a positional argument
		args = append(args, fmt.Sprintf(fileRefMessageFormat, promptPath))
	case PromptFileArg:
		// Add the prompt flag and file path
		if a.promptFlag != "" {
			args = append(args, a.promptFlag, promptPath)
		}
	case Stdin:
		// No additional args needed - caller is responsible for stdin setup
	}

	args = append(args, a.extraArgs...)

	// Create command - do not set stdout/stderr/stdin (caller's responsibility)
	cmd := exec.Command(a.binary, args...)

	return cmd, nil
}
