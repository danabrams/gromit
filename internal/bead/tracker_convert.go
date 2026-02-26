package bead

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/tracker"
)

const defaultTrackerCreatePriority = 1

// beadToItem converts a bd bead into a tracker item with rich metadata.
func beadToItem(b *Bead) *tracker.Item {
	if b == nil {
		return nil
	}

	metadata := map[string]string{
		"priority": fmt.Sprintf("%d", b.Priority),
	}
	addStringMetadata(metadata, "status", b.Status)
	addStringMetadata(metadata, "owner", b.Owner)
	addStringMetadata(metadata, "parent", b.Parent)
	addStringMetadata(metadata, "type", b.Type)
	addStringMetadata(metadata, "close_reason", b.CloseReason)
	addStringMetadata(metadata, "acceptance_criteria", b.AcceptanceCriteria)

	if encoded, ok := encodeJSONIfNonEmpty(b.Labels); ok {
		metadata["labels"] = encoded
	}
	if encoded, ok := encodeJSONIfNonEmpty(b.ExpectedOutputs); ok {
		metadata["expected_outputs"] = encoded
	}
	if encoded, ok := encodeJSONIfNonEmpty(b.Dependencies); ok {
		metadata["dependencies"] = encoded
	}
	if encoded, ok := encodeJSONIfNonEmpty(b.BlockedBy); ok {
		metadata["blocked_by"] = encoded
	}
	if encoded, ok := encodeJSONIfNonEmpty(b.DependsOn); ok {
		metadata["depends_on"] = encoded
	}
	if b.DependencyCount != nil {
		metadata["dependency_count"] = fmt.Sprintf("%d", *b.DependencyCount)
	}
	if b.DependentCount != nil {
		metadata["dependent_count"] = fmt.Sprintf("%d", *b.DependentCount)
	}

	return &tracker.Item{
		ID:          b.ID,
		Title:       b.Title,
		Description: b.Description,
		Status:      b.Status,
		Metadata:    metadata,
	}
}

func addStringMetadata(metadata map[string]string, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	metadata[key] = value
}

func encodeJSONIfNonEmpty(v interface{}) (string, bool) {
	if v == nil {
		return "", false
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" || trimmed == "[]" {
		return "", false
	}
	return string(data), true
}

func createParamsFromRequest(req tracker.CreateRequest) (int, []string, []string, string, error) {
	priority := defaultTrackerCreatePriority
	var labels []string
	var expectedOutputs []string
	var parent string

	if req.Metadata != nil {
		if raw, ok := req.Metadata["priority"]; ok {
			parsed, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil {
				return 0, nil, nil, "", fmt.Errorf("invalid priority %q: %w", raw, err)
			}
			if parsed < minPriority || parsed > maxPriority {
				return 0, nil, nil, "", fmt.Errorf("priority %d must be between %d and %d", parsed, minPriority, maxPriority)
			}
			priority = parsed
		}

		if raw, ok := req.Metadata["labels"]; ok {
			parsed, err := parseStringList(raw)
			if err != nil {
				return 0, nil, nil, "", fmt.Errorf("invalid labels metadata: %w", err)
			}
			labels = parsed
		}

		if raw, ok := req.Metadata["expected_outputs"]; ok {
			parsed, err := parseStringList(raw)
			if err != nil {
				return 0, nil, nil, "", fmt.Errorf("invalid expected_outputs metadata: %w", err)
			}
			expectedOutputs = parsed
		}

		if raw, ok := req.Metadata["parent"]; ok {
			parent = strings.TrimSpace(raw)
		}
	}

	return priority, labels, expectedOutputs, parent, nil
}

// TrackerItemToBead converts a tracker.Item back into a bead, if possible.
func TrackerItemToBead(item *tracker.Item) (*Bead, error) {
	if item == nil {
		return nil, nil
	}

	b := &Bead{
		ID:          item.ID,
		Title:       item.Title,
		Description: item.Description,
		Status:      item.Status,
	}
	if item.Metadata == nil {
		item.Metadata = map[string]string{}
	}

	if priority, ok := parseInt(item.Metadata["priority"]); ok {
		b.Priority = priority
	}
	if labels, err := parseStringList(item.Metadata["labels"]); err == nil && len(labels) > 0 {
		b.Labels = labels
	}
	if outputs, err := parseStringList(item.Metadata["expected_outputs"]); err == nil && len(outputs) > 0 {
		b.ExpectedOutputs = outputs
	}
	if parent := strings.TrimSpace(item.Metadata["parent"]); parent != "" {
		b.Parent = parent
	}
	if typ := strings.TrimSpace(item.Metadata["type"]); typ != "" {
		b.Type = typ
	}
	if owner := strings.TrimSpace(item.Metadata["owner"]); owner != "" {
		b.Owner = owner
	}
	if close := strings.TrimSpace(item.Metadata["close_reason"]); close != "" {
		b.CloseReason = close
	}
	if acceptance := strings.TrimSpace(item.Metadata["acceptance_criteria"]); acceptance != "" {
		b.AcceptanceCriteria = acceptance
	}
	if deps, ok := parseDependencies(item.Metadata["dependencies"]); ok {
		b.Dependencies = deps
	}
	if blocked, ok := parseDependencies(item.Metadata["blocked_by"]); ok {
		b.BlockedBy = blocked
	}
	if depends, ok := parseDependencies(item.Metadata["depends_on"]); ok {
		b.DependsOn = depends
	}
	if count, ok := parseInt(item.Metadata["dependency_count"]); ok {
		b.DependencyCount = &count
	}
	if count, ok := parseInt(item.Metadata["dependent_count"]); ok {
		b.DependentCount = &count
	}

	if err := prepareBeadForUse(b); err != nil {
		return nil, err
	}
	return b, nil
}

func parseInt(raw string) (int, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	val, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false
	}
	return val, true
}

func parseDependencies(raw string) ([]Dependency, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, false
	}
	var deps []Dependency
	if err := json.Unmarshal([]byte(trimmed), &deps); err != nil {
		return nil, false
	}
	if len(deps) == 0 {
		return nil, false
	}
	return deps, true
}

func parseStringList(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var decoded []string
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded, nil
		}
	}

	parts := strings.Split(trimmed, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	if len(result) == 0 {
		return nil, nil
	}

	return result, nil
}
