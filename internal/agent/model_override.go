package agent

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

	if a.Name() != "claude" {
		return ModelOverrideResult{}
	}

	cli, ok := a.(*cliAgent)
	if !ok {
		return ModelOverrideResult{}
	}

	newFlags := append([]string{}, cli.flags...)
	newFlags = append(newFlags, "--model", model)

	return ModelOverrideResult{
		Agent: &cliAgent{
			name:           cli.name,
			binary:         cli.binary,
			flags:          newFlags,
			promptDelivery: cli.promptDelivery,
			promptFlag:     cli.promptFlag,
			extraArgs:      append([]string{}, cli.extraArgs...),
			commandFn:      cli.commandFn,
			runFn:          cli.runFn,
		},
	}
}
