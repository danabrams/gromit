package stage

import "testing"

func TestStageRequestConstruction(t *testing.T) {
    req := StageRequest{
        Bead: BeadInfo{ID: "spec"},
    }

    if req.Bead.ID != "spec" {
        t.Fatalf("expected bead ID to be spec, got %s", req.Bead.ID)
    }
}
