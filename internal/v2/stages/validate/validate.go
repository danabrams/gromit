package validate

import "github.com/danabrams/gromit/internal/config"

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "validate"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "validate:default"
	}
	return profile + ":" + "validate"
}
