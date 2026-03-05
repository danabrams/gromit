package accept

import "github.com/danabrams/gromit/internal/config"

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "accept"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "accept:default"
	}
	return profile + ":" + "accept"
}
