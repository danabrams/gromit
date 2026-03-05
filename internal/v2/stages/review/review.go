package review

import "github.com/danabrams/gromit/internal/config"

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "review"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "review:default"
	}
	return profile + ":" + "review"
}
