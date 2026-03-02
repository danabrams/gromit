package runner

import "testing"

func TestSkipTrackerRegisterBead(t *testing.T) {
    t.Run("new bead", func(t *testing.T) {
        s := newSkipTracker()
        skip, stop := s.registerBead("bead-a")
        if skip || stop {
            t.Fatalf("unexpected skip state: skip=%v stop=%v", skip, stop)
        }
    })

    t.Run("repeat until stop", func(t *testing.T) {
        s := newSkipTracker()
        s.registerBead("bead-a")
        skip, stop := s.registerBead("bead-a")
        if !skip || !stop {
            t.Fatalf("expected skip=true stop=true, got skip=%v stop=%v", skip, stop)
        }
    })

    t.Run("multiple beads require aggregate skips", func(t *testing.T) {
        s := newSkipTracker()
        s.registerBead("bead-a")
        s.registerBead("bead-b")

        skip, stop := s.registerBead("bead-a")
        if !skip || stop {
            t.Fatalf("expected skip=true stop=false after first reoffer, got skip=%v stop=%v", skip, stop)
        }

        skip, stop = s.registerBead("bead-b")
        if !skip || !stop {
            t.Fatalf("expected skip=true stop=true after enough skips, got skip=%v stop=%v", skip, stop)
        }
    })
}
