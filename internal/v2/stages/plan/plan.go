package plan

import "github.com/danabrams/gromit/internal/config"

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "plan"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "plan:default"
	}
	return profile + ":" + "plan"
}
