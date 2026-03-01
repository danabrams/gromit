package retro

import "testing"

func TestCompareFrictionResolutions(t *testing.T) {
	t.Run("classifies resolutions across reports", func(t *testing.T) {
		previous := &WorkmanshipReport{
			FrictionClusters: []FrictionCluster{
				{Area: "pkg/resolved", LearningCount: 1},
				{Area: "pkg/persisting", LearningCount: 2},
			},
		}
		current := []FrictionCluster{
			{Area: "pkg/new", LearningCount: 3},
			{Area: "pkg/persisting", LearningCount: 4},
		}

		resolutions := CompareFriction(current, previous)
		if len(resolutions) != 3 {
			t.Fatalf("expected 3 resolutions, got %d", len(resolutions))
		}

		byArea := make(map[string]FrictionResolution)
		for _, resolution := range resolutions {
			byArea[resolution.Area] = resolution
		}

		check := func(area, status string, prev, current int) {
			resolution, ok := byArea[area]
			if !ok {
				t.Fatalf("missing resolution for %s", area)
			}
			if resolution.Status != status {
				t.Fatalf("expected %s to be %s, got %s", area, status, resolution.Status)
			}
			if resolution.PreviousCount != prev {
				t.Fatalf("expected %s previous count %d, got %d", area, prev, resolution.PreviousCount)
			}
			if resolution.CurrentCount != current {
				t.Fatalf("expected %s current count %d, got %d", area, current, resolution.CurrentCount)
			}
		}

		check("pkg/new", "new", 0, 3)
		check("pkg/persisting", "persisting", 2, 4)
		check("pkg/resolved", "resolved", 1, 0)
	})

	t.Run("nil previous report yields only new areas", func(t *testing.T) {
		current := []FrictionCluster{{Area: "pkg/solo", LearningCount: 5}}
		resolutions := CompareFriction(current, nil)
		if len(resolutions) != 1 {
			t.Fatalf("expected 1 resolution, got %d", len(resolutions))
		}
		resolution := resolutions[0]
		if resolution.Status != "new" {
			t.Fatalf("expected new status, got %s", resolution.Status)
		}
		if resolution.PreviousCount != 0 || resolution.CurrentCount != 5 {
			t.Fatalf("unexpected counts for new area: %+v", resolution)
		}
	})
}
