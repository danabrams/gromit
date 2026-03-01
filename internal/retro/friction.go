package retro

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/learnings"
)

const (
	// minFrictionClusterSize defines the minimum learnings required to consider an area as friction.
	minFrictionClusterSize = 2
)

const crossCuttingArea = "cross-cutting"

var areaPattern = regexp.MustCompile(`[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+`)

func clusterByArea(learnings []learnings.Learning, minClusterSize int) []FrictionCluster {
	if len(learnings) == 0 {
		return nil
	}
	if minClusterSize <= 0 {
		minClusterSize = 1
	}

	clusters := make(map[string]*FrictionCluster)
	for _, learning := range learnings {
		area := extractArea(learning.Content)
		if area == "" {
			area = crossCuttingArea
		}
		cluster, exists := clusters[area]
		if !exists {
			cluster = &FrictionCluster{
				Area:         area,
				EarliestDate: learning.Date,
				LatestDate:   learning.Date,
				Categories:   make(map[string]int),
			}
			clusters[area] = cluster
		}
		cluster.LearningCount++
		if !learning.Date.IsZero() {
			if cluster.EarliestDate.IsZero() || learning.Date.Before(cluster.EarliestDate) {
				cluster.EarliestDate = learning.Date
			}
			if cluster.LatestDate.IsZero() || learning.Date.After(cluster.LatestDate) {
				cluster.LatestDate = learning.Date
			}
		}
		if learning.Category != "" {
			cluster.Categories[learning.Category]++
		}
	}

	result := make([]FrictionCluster, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster.LearningCount < minClusterSize {
			continue
		}
		result = append(result, *cluster)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].LearningCount != result[j].LearningCount {
			return result[i].LearningCount > result[j].LearningCount
		}
		return result[i].Area < result[j].Area
	})

	return result
}

func extractArea(content string) string {
	matches := areaPattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return crossCuttingArea
	}

	bestArea := ""
	counts := make(map[string]int)
	for _, match := range matches {
		normalized := normalizeArea(match)
		if normalized == "" {
			continue
		}
		counts[normalized]++
		if bestArea == "" || counts[normalized] > counts[bestArea] || (counts[normalized] == counts[bestArea] && normalized < bestArea) {
			bestArea = normalized
		}
	}

	if bestArea == "" {
		return crossCuttingArea
	}
	return bestArea
}

func normalizeArea(raw string) string {
	area := strings.TrimSpace(raw)
	area = strings.Trim(area, ".,;:!()[]\"'")
	if area == "" || strings.Contains(area, "://") {
		return ""
	}
	if !strings.Contains(area, "/") {
		return ""
	}
	if ext := filepath.Ext(area); ext != "" {
		area = strings.TrimSuffix(area, ext)
		area = filepath.Dir(area)
	}
	area = strings.Trim(area, "/")
	if area == "" {
		return ""
	}
	return area
}
