package names

import "github.com/danabrams/gromit/internal/config"

// Describe builds a unique stage identifier using the project profile and stage name.
func Describe(stage string, cfg *config.Config) string {
	if cfg == nil {
		return stage
	}
	profile := cfg.Project.Profile
	if profile == "" {
		return stage + ":default"
	}
	return profile + ":" + stage
}
