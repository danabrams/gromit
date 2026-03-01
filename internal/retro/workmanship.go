package retro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// WorkmanshipReport summarizes the last stored friction findings.
type WorkmanshipReport struct {
	FrictionClusters []FrictionCluster `json:"friction_clusters,omitempty"`
}

// WorkmanshipHistory persists the latest workmanship report for future retros.
type WorkmanshipHistory struct {
	Report WorkmanshipReport `json:"report"`
}

// FindPreviousFriction returns the stored friction cluster for area, if any.
func (h *WorkmanshipHistory) FindPreviousFriction(area string) *FrictionCluster {
	if h == nil {
		return nil
	}
	for i := range h.Report.FrictionClusters {
		cluster := &h.Report.FrictionClusters[i]
		if cluster.Area == area {
			return cluster
		}
	}
	return nil
}

// LoadWorkmanshipHistory reads the history at path.
func LoadWorkmanshipHistory(path string) (*WorkmanshipHistory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workmanship history: %w", err)
	}
	var history WorkmanshipHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("parse workmanship history: %w", err)
	}
	return &history, nil
}

// SaveWorkmanshipHistory writes history to path in JSON format.
func SaveWorkmanshipHistory(path string, history *WorkmanshipHistory) error {
	if history == nil {
		history = &WorkmanshipHistory{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create history directory: %w", err)
	}
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workmanship history: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write workmanship history: %w", err)
	}
	return nil
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
