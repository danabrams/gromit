package agent

import (
	"fmt"
)

// ModelOverrideResult carries the outcome of a model override attempt.
type ModelOverrideResult struct {
	Agent   Agent
	Warning string
}

// TryOverrideModel attempts to inject a model override for supported agents.
func TryOverrideModel(a Agent, model string) ModelOverrideResult {
	if a == nil || model == "" {
		return ModelOverrideResult{}
	}

	cli, ok := a.(*cliAgent)
	if !ok {
		return ModelOverrideResult{
			Warning: fmt.Sprintf("model override not supported for agent %q", a.Name()),
		}
	}

	if a.Name() == "claude" {
		newFlags := append([]string{}, cli.flags...)
		newFlags = append(newFlags, "--model", model)
		return ModelOverrideResult{Agent: cloneCliAgent(cli, newFlags, cli.extraArgs)}
	}

	newExtraArgs := append([]string{}, cli.extraArgs...)
	newExtraArgs = append(newExtraArgs, "--model", model)
	return ModelOverrideResult{Agent: cloneCliAgent(cli, cli.flags, newExtraArgs)}
}

func cloneCliAgent(base *cliAgent, flags, extraArgs []string) *cliAgent {
	clonedFlags := append([]string{}, flags...)
	clonedExtra := append([]string{}, extraArgs...)
	return &cliAgent{
		name:           base.name,
		binary:         base.binary,
		flags:          clonedFlags,
		promptDelivery: base.promptDelivery,
		promptFlag:     base.promptFlag,
		extraArgs:      clonedExtra,
		commandFn:      base.commandFn,
		runFn:          base.runFn,
	}
}
