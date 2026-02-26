package methodology

// RunnerAdapter adapts runner command lists for acceptance, TDD, and validation phases.
type RunnerAdapter interface {
	Acceptance(commands []string, touchedPackages []string) []string
	TDD(commands []string, touchedPackages []string) []string
	Validation(commands []string, touchedPackages []string) []string
}
