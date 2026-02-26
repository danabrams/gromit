package methodology

// PassthroughAdapter returns commands unchanged for every phase.
type PassthroughAdapter struct{}

var _ RunnerAdapter = PassthroughAdapter{}

func (PassthroughAdapter) Acceptance(commands []string, touchedPackages []string) []string {
	return commands
}

func (PassthroughAdapter) TDD(commands []string, touchedPackages []string) []string {
	return commands
}

func (PassthroughAdapter) Validation(commands []string, touchedPackages []string) []string {
	return commands
}
