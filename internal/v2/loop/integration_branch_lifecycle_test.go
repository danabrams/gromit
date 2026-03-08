package loop

import (
    "os/exec"
    "testing"
)

func TestIntegrationBranchLifecycleFailurePreservesBranch(t *testing.T) {
    t.Parallel()

    if _, err := exec.LookPath("git"); err != nil {
        t.Skip("git not available")
    }

    const specID = "spec-branch-lifecycle-failure"
    repoRoot := t.TempDir()
    initGitRepo(t, repoRoot)

    verifyFailureBranchLifecycle(t, repoRoot, specID)
}
