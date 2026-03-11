package executor

// GitClient abstracts git operations for testability.
type GitClient interface {
	DiffFiles(workDir string) ([]string, error)
}

// InspectChanges returns the list of files modified in the given worktree.
func InspectChanges(git GitClient, workDir string) ([]string, error) {
	files, err := git.DiffFiles(workDir)
	if err != nil {
		return nil, err
	}
	if files == nil {
		return []string{}, nil
	}
	return files, nil
}
