package build

import "github.com/danabrams/gromit/internal/config"

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "build"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "build:default"
	}
	return profile + ":" + "build"
}
