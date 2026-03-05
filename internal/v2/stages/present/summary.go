package present

import (
    "strings"

    v2review "github.com/danabrams/gromit/internal/v2/review"
)

// FormatOutOfScopeSummary formats a product owner summary of review findings outside the current scope.
func FormatOutOfScopeSummary(findings []v2review.Finding) string {
    if len(findings) == 0 {
        return "Product owner summary: No out-of-scope review findings were observed."
    }

    var builder strings.Builder
    builder.WriteString("Product Owner summary: Out-of-scope review findings were observed.\n")
    for _, finding := range findings {
        builder.WriteString("- " + finding.Title)
        if strings.TrimSpace(finding.Description) != "" {
            builder.WriteString(": " + finding.Description)
        }
        builder.WriteString("\n")
        if len(finding.AffectedFiles) > 0 {
            builder.WriteString("  Affected files: " + strings.Join(finding.AffectedFiles, ", ") + "\n")
        }
    }
    return builder.String()
}
