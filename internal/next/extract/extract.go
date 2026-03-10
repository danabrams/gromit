package extract

import "github.com/danabrams/gromit/internal/next/fact"

// Extractor produces facts by inspecting a repository.
type Extractor interface {
	Name() string
	Extract(repoPath string) ([]fact.Fact, error)
}
