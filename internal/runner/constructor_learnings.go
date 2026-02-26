package runner

import (
	"sort"

	"github.com/danabrams/gromit/internal/provider"
)

func selectLearningsProvider(configuredName string, providers map[string]provider.Provider) provider.Provider {
	if len(providers) == 0 {
		return nil
	}
	if configuredName != "" {
		if p, ok := providers[configuredName]; ok {
			return p
		}
	}
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return providers[names[0]]
}
