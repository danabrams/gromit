package present

import "github.com/danabrams/gromit/internal/config"

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "present"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "present:default"
	}
	return profile + ":" + "present"
}
