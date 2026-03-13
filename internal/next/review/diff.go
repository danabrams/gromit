package review

// DiffProvider computes diffs against a base branch.
type DiffProvider interface {
	Diff(baseBranch string) (string, error)
}
