package agent

// ForwardModelToAgent accepts a resolved agent and a requested model string.
// It returns either an overridden agent with model args injected (for known presets)
// or the original agent with a warning message (for unsupported agents).
//
// Returns:
//   - agent: The modified agent (with model forwarded) or original agent
//   - warning: Empty string if successful, warning message if forwarding was skipped
func ForwardModelToAgent(agent Agent, model string) (Agent, string) {
	// If no model requested, return agent unchanged with no warning
	if model == "" {
		return agent, ""
	}

	// Handle codex preset
	if agent.Name() == "codex" {
		// Codex preset uses --model flag
		return forwardModelToCodex(agent, model)
	}

	// Handle gemini preset
	if agent.Name() == "gemini" {
		// Gemini preset uses --model flag
		return forwardModelToGemini(agent, model)
	}

	// For other agents (claude or custom), return warning
	return agent, "model forwarding not supported for agent " + agent.Name()
}

// forwardModelToCodex creates a new codex agent with the model flag injected
func forwardModelToCodex(agent Agent, model string) (Agent, string) {
	a, ok := agent.(*cliAgent)
	if !ok {
		// Shouldn't happen for codex preset, but fall back to warning
		return agent, "unable to forward model to " + agent.Name()
	}

	// Clone the agent with additional model args
	newExtraArgs := make([]string, len(a.extraArgs)+2)
	copy(newExtraArgs, a.extraArgs)
	newExtraArgs[len(a.extraArgs)] = "--model"
	newExtraArgs[len(a.extraArgs)+1] = model

	// Create new agent with modified extraArgs
	modifiedAgent := &cliAgent{
		name:           a.name,
		binary:         a.binary,
		flags:          a.flags,
		promptDelivery: a.promptDelivery,
		promptFlag:     a.promptFlag,
		extraArgs:      newExtraArgs,
		commandFn:      a.commandFn,
		runFn:          a.runFn,
	}

	return modifiedAgent, ""
}

// forwardModelToGemini creates a new gemini agent with the model flag injected
func forwardModelToGemini(agent Agent, model string) (Agent, string) {
	a, ok := agent.(*cliAgent)
	if !ok {
		// Shouldn't happen for gemini preset, but fall back to warning
		return agent, "unable to forward model to " + agent.Name()
	}

	// Clone the agent with additional model args
	newExtraArgs := make([]string, len(a.extraArgs)+2)
	copy(newExtraArgs, a.extraArgs)
	newExtraArgs[len(a.extraArgs)] = "--model"
	newExtraArgs[len(a.extraArgs)+1] = model

	// Create new agent with modified extraArgs
	modifiedAgent := &cliAgent{
		name:           a.name,
		binary:         a.binary,
		flags:          a.flags,
		promptDelivery: a.promptDelivery,
		promptFlag:     a.promptFlag,
		extraArgs:      newExtraArgs,
		commandFn:      a.commandFn,
		runFn:          a.runFn,
	}

	return modifiedAgent, ""
}
