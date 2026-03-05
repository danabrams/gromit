package decompose

import "github.com/danabrams/gromit/internal/config"

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "decompose"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "decompose:default"
	}
	return profile + ":" + "decompose"
}
