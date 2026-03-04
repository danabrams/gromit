package config

// NormalizeNilFields ensures nil slices and maps are replaced with empty
// instances. This prevents issues with downstream code that marshals to JSON
// (nil -> "null" vs [] -> "[]") and ensures consistent behavior.
// See CLAUDE.md nil-field normalization visibility convention:
// Config crosses package boundaries so the helper stays exported.
func (c *Config) NormalizeNilFields() {
	if c.Escalation.Chain == nil {
		c.Escalation.Chain = []string{}
	}
	if c.Validation.Commands == nil {
		c.Validation.Commands = []string{}
	}
	if c.Validation.FastCommands == nil {
		c.Validation.FastCommands = []string{}
	}
	if c.Validation.FullCommands == nil {
		c.Validation.FullCommands = []string{}
	}
	if c.Validation.MandatoryCommands == nil {
		c.Validation.MandatoryCommands = []string{}
	}
	if c.Andon.HardStops.BulkDeleteAllowlist == nil {
		c.Andon.HardStops.BulkDeleteAllowlist = []string{}
	}
	if c.Preflight.Tools == nil {
		c.Preflight.Tools = []string{}
	}
	if c.Claude.Flags == nil {
		c.Claude.Flags = []string{}
	}
	if c.Claude.ModelTimeouts == nil {
		c.Claude.ModelTimeouts = make(map[string]ModelTimeoutOverrides)
	}
	if c.Models.Labels == nil {
		c.Models.Labels = make(map[string]string)
	}
	if c.Agents.Definitions == nil {
		c.Agents.Definitions = make(map[string]AgentDefinition)
	}
	for name, def := range c.Agents.Definitions {
		if def.Flags == nil {
			def.Flags = []string{}
			c.Agents.Definitions[name] = def
		}
	}
	if c.Providers == nil {
		c.Providers = make(map[string]ProviderDef)
	}
	for name, def := range c.Providers {
		if def.Flags == nil {
			def.Flags = []string{}
		}
		if def.Models == nil {
			def.Models = make(map[string]string)
		}
		if def.ReasoningEffort == nil {
			def.ReasoningEffort = make(map[string]string)
		}
		def.ModelCosts = normalizeProviderModelCosts(def.ModelCosts)
		c.Providers[name] = def
	}
	if c.Routing.PhasePreferences == nil {
		c.Routing.PhasePreferences = make(map[string]string)
	}
	if c.Routing.Ratio == nil {
		c.Routing.Ratio = make(map[string]int)
	}
	if c.QualityGates == nil {
		c.QualityGates = &QualityGatesConfig{}
	}
	if c.QualityGates.Regression == nil {
		c.QualityGates.Regression = &RegressionGateConfig{}
	}
	if c.QualityGates.Wiring == nil {
		c.QualityGates.Wiring = &WiringGateConfig{}
	}
}

func normalizeProviderModelCosts(costs map[string]*ModelCost) map[string]*ModelCost {
	if costs == nil {
		return make(map[string]*ModelCost)
	}
	return costs
}
