package methodology

// GoAdapter implements RunnerAdapter behavior for Go commands.
type GoAdapter struct{}

var _ RunnerAdapter = GoAdapter{}

func (GoAdapter) Acceptance(commands []string, touchedPackages []string) []string {
	return AcceptanceCommands(commands, touchedPackages)
}

func (GoAdapter) TDD(commands []string, touchedPackages []string) []string {
	return commands
}

func (GoAdapter) Validation(commands []string, touchedPackages []string) []string {
	return commands
}
