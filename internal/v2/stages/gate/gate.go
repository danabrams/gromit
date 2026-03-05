package gate

import "github.com/danabrams/gromit/internal/config"

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "gate"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "gate:default"
	}
	return profile + ":" + "gate"
}
