package testutil

// FakeGit returns configured diff output.
type FakeGit struct {
	DiffOutput string
	DiffErr    error
}

func (g *FakeGit) Diff(_ string) (string, error) {
	return g.DiffOutput, g.DiffErr
}
