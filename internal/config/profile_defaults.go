package config

import "strings"

// ProfileDefaults describes configuration overrides derived from a profile selector.
type ProfileDefaults struct {
	ValidationCommands     []string
	PreflightCompileCommand string
}

var (
	profileCatalog = map[string]ProfileDefaults{
		"go": {
			ValidationCommands: []string{"go test", "go build", "go vet"},
		},
		"node": {
			ValidationCommands: []string{"npm test", "npm run build"},
		},
		"python": {
			ValidationCommands: []string{"pytest"},
		},
		"custom": {},
	}
	profileNames = []string{"go", "node", "python", "custom"}
)

// ProfileForName looks up the defaults associated with a named profile selector
// and reports whether it is a recognized profile. The returned slice is
// copied to avoid accidental mutation of shared defaults.
func ProfileForName(name string) (ProfileDefaults, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	defaults, ok := profileCatalog[normalized]
	if !ok {
		return ProfileDefaults{}, false
	}
	if len(defaults.ValidationCommands) > 0 {
		copied := append([]string(nil), defaults.ValidationCommands...)
		defaults.ValidationCommands = copied
	}
	return defaults, true
}
