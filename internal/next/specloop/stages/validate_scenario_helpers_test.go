package stages

type validateScenarioFakeGitOps struct {
	removeErr       error
	recoverErr      error
	capturedRepoDir string
	recoverBranch   string
	removeCalled    bool
	recoverCalled   bool
	recoveredPath   string
}

func (f *validateScenarioFakeGitOps) CreateWorktree(repoDir, branch string) (string, error) {
	return "", nil
}

func (f *validateScenarioFakeGitOps) RemoveWorktree(path string) error {
	f.removeCalled = true
	return f.removeErr
}

func (f *validateScenarioFakeGitOps) RecoverWorktree(repoDir, branch string) (string, error) {
	f.recoverCalled = true
	f.capturedRepoDir = repoDir
	f.recoverBranch = branch
	// If recoveredPath was not set by the test, use a default
	if f.recoveredPath == "" {
		f.recoveredPath = "/recovered/worktree"
	}
	return f.recoveredPath, f.recoverErr
}

func (f *validateScenarioFakeGitOps) CommitAll(workDir, message string) error {
	return nil
}

// fakeGitOps is a test double for GitOps used in scenario validation tests.
var _ GitOps = (*validateScenarioFakeGitOps)(nil)
