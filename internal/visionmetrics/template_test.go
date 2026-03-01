package visionmetrics

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestPRTemplateIncludesRequiredVisionMetricsFields(t *testing.T) {
    path := filepath.Join("..", "..", ".github", "pull_request_template.md")
    data, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("unable to read template: %v", err)
    }

    content := string(data)
    required := []string{
        "# Vision Metrics",
        "spec_id:",
        "cycle_start_trigger_at:",
        "cycle_end_presented_at:",
        "review_outcome:",
        "human_tactical_intervention:",
        "human_debugging_intervention:",
        "escaped_regression_within_7d:",
    }

    for _, substring := range required {
        if !strings.Contains(content, substring) {
            t.Fatalf("template missing %q", substring)
        }
    }
}
