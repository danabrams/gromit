package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type reviewLogRecord struct {
	Type          string   `json:"type"`
	ReviewType    string   `json:"review_type"`
	BeadID        string   `json:"bead_id"`
	FixCategories []string `json:"fix_categories"`
}

type categoryCount struct {
	category string
	count    int
}

// ReadRecurringReviewFixCategories returns top recurring review fix categories.
// When specID is non-empty, only reviews for beads in the same spec are considered.
func ReadRecurringReviewFixCategories(logsDir, currentBeadID, specID string, minOccurrences, maxCategories int) ([]string, error) {
	if maxCategories <= 0 {
		return []string{}, nil
	}
	if minOccurrences <= 0 {
		minOccurrences = 1
	}

	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	specByBead := make(map[string]string)
	for _, f := range files {
		entries, err := readLogFile(f)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.BeadID == "" {
				continue
			}
			specByBead[entry.BeadID] = entry.SpecID
		}
	}

	counts := make(map[string]int)
	for _, f := range files {
		reviews, err := readReviewLogFile(f)
		if err != nil {
			continue
		}
		for _, review := range reviews {
			if review.BeadID == "" || review.BeadID == currentBeadID {
				continue
			}
			if specID != "" && specByBead[review.BeadID] != specID {
				continue
			}
			for _, category := range review.FixCategories {
				if category == "" {
					continue
				}
				counts[category]++
			}
		}
	}

	ranked := make([]categoryCount, 0, len(counts))
	for category, count := range counts {
		if count < minOccurrences {
			continue
		}
		ranked = append(ranked, categoryCount{category: category, count: count})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count == ranked[j].count {
			return ranked[i].category < ranked[j].category
		}
		return ranked[i].count > ranked[j].count
	})

	if len(ranked) > maxCategories {
		ranked = ranked[:maxCategories]
	}

	result := make([]string, 0, len(ranked))
	for _, entry := range ranked {
		result = append(result, entry.category)
	}
	return result, nil
}

func readReviewLogFile(path string) ([]reviewLogRecord, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	return decodeReviewLogs(dec), nil
}

func decodeReviewLogs(dec *json.Decoder) []reviewLogRecord {
	reviews := []reviewLogRecord{}
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			break
		}

		var candidate reviewLogRecord
		if err := json.Unmarshal(raw, &candidate); err != nil {
			continue
		}
		if candidate.Type != "review" {
			continue
		}
		if candidate.FixCategories == nil {
			candidate.FixCategories = []string{}
		}
		reviews = append(reviews, candidate)
	}
	return reviews
}
