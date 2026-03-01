package visionmetrics

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	visionMetricsHeadingRe = regexp.MustCompile(`^#{1,6}\s*Vision Metrics\s*$`)
	keyValuePairRe         = regexp.MustCompile(`^([\w_]+)\s*:\s*(.*)$`)
)

// ParseFromPRBody extracts vision metrics metadata from a PR body and
// assembles it into a cycle record suitable for validation.
func ParseFromPRBody(body string) (Record, error) {
	lines, err := extractVisionMetricsLines(body)
	if err != nil {
		return Record{}, err
	}

	fields := make(map[string]string, len(lines))
	for _, line := range lines {
		matches := keyValuePairRe.FindStringSubmatch(line)
		if len(matches) != 3 {
			return Record{}, fmt.Errorf("malformed Vision Metrics line: %q", line)
		}
		key := strings.TrimSpace(matches[1])
		value := strings.TrimSpace(matches[2])
		fields[key] = value
	}

	return recordFromFields(fields)
}

func extractVisionMetricsLines(body string) ([]string, error) {
	lines := strings.Split(body, "\n")
	headingIdx := -1
	for i, raw := range lines {
		if visionMetricsHeadingRe.MatchString(strings.TrimSpace(raw)) {
			headingIdx = i + 1
			break
		}
	}
	if headingIdx == -1 {
		return nil, fmt.Errorf("Vision Metrics block not found in PR body")
	}

	var block []string
	started := false
	for i := headingIdx; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if keyValuePairRe.MatchString(trimmed) {
			block = append(block, trimmed)
			started = true
			continue
		}
		if !started {
			return nil, fmt.Errorf("malformed Vision Metrics block: first non-empty line %q is not key:value", trimmed)
		}
		break
	}

	if len(block) == 0 {
		return nil, fmt.Errorf("Vision Metrics block is empty")
	}

	return block, nil
}

func recordFromFields(fields map[string]string) (Record, error) {
	rec := Record{}
	rec.SpecID = strings.TrimSpace(fields[FieldSpecID])

	if err := parseTimeField(fields, FieldCycleStartTriggerAt, &rec.CycleStartTriggerAt); err != nil {
		return Record{}, err
	}
	if err := parseTimeField(fields, FieldCycleEndPresentedAt, &rec.CycleEndPresentedAt); err != nil {
		return Record{}, err
	}

	if val := strings.TrimSpace(fields[FieldReviewOutcome]); val != "" {
		rec.ReviewOutcome = ReviewOutcome(val)
	}
	if val := strings.TrimSpace(fields["review_rationale"]); val != "" {
		rec.ReviewRationale = val
	}
	if val := strings.TrimSpace(fields[FieldHumanTacticalIntervention]); val != "" {
		rec.HumanTacticalIntervention = YesNo(val)
	}
	if val := strings.TrimSpace(fields[FieldHumanDebuggingIntervention]); val != "" {
		rec.HumanDebuggingIntervention = YesNo(val)
	}
	if val := strings.TrimSpace(fields[FieldEscapedRegressionWithin7D]); val != "" {
		rec.EscapedRegressionWithin7D = YesNo(val)
	}

	return rec, nil
}

func parseTimeField(fields map[string]string, key string, dest *time.Time) error {
	if val, ok := fields[key]; ok && strings.TrimSpace(val) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(val))
		if err != nil {
			return fmt.Errorf("invalid %s format: %w", key, err)
		}
		*dest = t
	}
	return nil
}
