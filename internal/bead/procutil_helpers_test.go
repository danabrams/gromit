package bead

import "testing"

func TestRestoreBeadProcutilFnsRegistersCleanup(t *testing.T) {

    // Register a cleanup after the helper so that the helper's cleanup runs
    // before this assertion (t.Cleanup rolls back in reverse order).
    t.Cleanup(func() {
        if waitForProcessCapacityFn == nil {
            t.Fatalf("wait hook still nil after cleanup")
        }
        if killDescendantsOnCancelFn == nil {
            t.Fatalf("kill hook still nil after cleanup")
        }
        if reapProcessTreeFn == nil {
            t.Fatalf("reap hook still nil after cleanup")
        }
        if subprocessEnvFn == nil {
            t.Fatalf("env hook still nil after cleanup")
        }
        if resolveBeadsDirFn == nil {
            t.Fatalf("resolve hook still nil after cleanup")
        }
    })

    restoreBeadProcutilFns(t)

    waitForProcessCapacityFn = nil
    killDescendantsOnCancelFn = nil
    reapProcessTreeFn = nil
    subprocessEnvFn = nil
    resolveBeadsDirFn = nil
}
