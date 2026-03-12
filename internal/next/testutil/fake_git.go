package testutil

// FakeGit returns configured diff output and records calls.
type FakeGit struct {
	DiffOutput string
	DiffErr    error
	DiffCalls  []string
}

func (g *FakeGit) Diff(baseBranch string) (string, error) {
	g.DiffCalls = append(g.DiffCalls, baseBranch)
	return g.DiffOutput, g.DiffErr
}
