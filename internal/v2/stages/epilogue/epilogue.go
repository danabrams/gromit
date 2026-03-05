package epilogue

import "github.com/danabrams/gromit/internal/config"

func Describe(cfg *config.Config) string {
	if cfg == nil {
		return "epilogue"
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return "epilogue:default"
	}
	return profile + ":" + "epilogue"
}
