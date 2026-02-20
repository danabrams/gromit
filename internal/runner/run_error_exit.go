package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const earlyExitPrompt = "Run end-of-loop command before exiting due to error? [y/N]: "

func (r *Runner) handleRunError(runErr error) error {
	if runErr == nil {
		return nil
	}
	if r == nil || r.cfg == nil {
		return runErr
	}
	if strings.TrimSpace(r.cfg.Loop.EndOfLoopCommand) == "" {
		return runErr
	}

	stdinStatFn := r.stdinStatFn
	if stdinStatFn == nil {
		stdinStatFn = os.Stdin.Stat
	}
	if !isInteractiveStdin(stdinStatFn) {
		return runErr
	}

	promptYesNoFn := r.promptYesNoFn
	if promptYesNoFn == nil {
		if !isInteractiveOutput(r.output) {
			return runErr
		}
		promptYesNoFn = func(question string) (bool, error) {
			return promptYesNo(os.Stdin, r.output, question)
		}
	}

	confirmed, err := promptYesNoFn(earlyExitPrompt)
	if err != nil {
		return runErr
	}
	if !confirmed {
		return runErr
	}

	if err := r.runEndOfLoopCommand(); err != nil {
		return errors.Join(runErr, fmt.Errorf("early-exit end-of-loop command: %w", err))
	}

	return runErr
}

func isInteractiveOutput(output io.Writer) bool {
	if output == nil {
		return false
	}
	if sw, ok := output.(*syncWriter); ok {
		output = sw.w
	}
	file, ok := output.(*os.File)
	if !ok || file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil || info == nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
