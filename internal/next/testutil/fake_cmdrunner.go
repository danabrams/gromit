package testutil

// CmdResult holds the output of a command execution.
type CmdResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// FakeCmdRunner returns pre-configured outputs for known commands.
type FakeCmdRunner struct {
	Outputs map[string]CmdResult
}

func (r *FakeCmdRunner) Run(cmd string) CmdResult {
	if result, ok := r.Outputs[cmd]; ok {
		return result
	}
	return CmdResult{Stderr: "unknown command: " + cmd, ExitCode: 1}
}
