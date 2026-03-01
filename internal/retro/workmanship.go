package retro

import "sort"

// WorkmanshipReport summarizes the last stored friction findings.
type WorkmanshipReport struct {
	FrictionClusters []FrictionCluster `json:"friction_clusters,omitempty"`
}

const (
	frictionStatusResolved   = "resolved"
	frictionStatusPersisting = "persisting"
	frictionStatusNew        = "new"
)

// CompareFriction classifies the fate of friction clusters between runs.
func CompareFriction(current []FrictionCluster, previous *WorkmanshipReport) []FrictionResolution {
	previousByArea := make(map[string]FrictionCluster)
	if previous != nil {
		for _, cluster := range previous.FrictionClusters {
			previousByArea[cluster.Area] = cluster
		}
	}

	seen := make(map[string]struct{}, len(current))
	resolutions := make([]FrictionResolution, 0, len(current)+len(previousByArea))

	for _, cluster := range current {
		status := frictionStatusNew
		previousCount := 0
		if prev, ok := previousByArea[cluster.Area]; ok {
			status = frictionStatusPersisting
			previousCount = prev.LearningCount
		}
		seen[cluster.Area] = struct{}{}
		resolutions = append(resolutions, FrictionResolution{
			Area:          cluster.Area,
			Status:        status,
			PreviousCount: previousCount,
			CurrentCount:  cluster.LearningCount,
		})
	}

	for area, prev := range previousByArea {
		if _, already := seen[area]; already {
			continue
		}
		resolutions = append(resolutions, FrictionResolution{
			Area:          area,
			Status:        frictionStatusResolved,
			PreviousCount: prev.LearningCount,
			CurrentCount:  0,
		})
	}

	sort.Slice(resolutions, func(i, j int) bool {
		return resolutions[i].Area < resolutions[j].Area
	})
	return resolutions
}
