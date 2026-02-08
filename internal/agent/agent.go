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

// Agent represents an AI agent that can be launched with a prompt
type Agent interface {
	// Name returns the agent's name
	Name() string
	// Launch executes the agent with the given prompt file path
	Launch(promptPath string) error
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
		args = append(args, fmt.Sprintf("Read and follow instructions in %s", promptPath))
	case PromptFileArg:
		// Add the prompt flag and file path
		if a.promptFlag != "" {
			args = append(args, a.promptFlag, promptPath)
		}
	case Stdin:
		// No additional args needed - we'll pipe stdin below
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
		cmd.Stdin = nil // Will be handled by StdinPipe or direct assignment
		// Use a simpler approach - write to stdin directly
		r, w, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}
		cmd.Stdin = r
		go func() {
			defer w.Close()
			w.Write(content)
		}()
	} else {
		cmd.Stdin = os.Stdin
	}

	// Run the command
	err := cmd.Run()

	// Treat exec.ExitError as graceful exit (agent returned non-zero)
	if exitErr, ok := err.(*exec.ExitError); ok {
		_ = exitErr // Agent exited with non-zero, but this is normal
		return nil
	}

	return err
}
