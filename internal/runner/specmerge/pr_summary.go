package specmerge

import (
    "context"
    "fmt"
    "strings"
)

const (
    defaultSpecExcerptLines = 6
    defaultDiffExcerptLines = 12
)

// PRSummaryInput captures the data used to build a PR summary body.
type PRSummaryInput struct {
    SpecName    string
    SpecContent string
    Diff        string
}

// BuildPRSummary renders a simple PR summary that highlights the spec context and diff.
func BuildPRSummary(ctx context.Context, input PRSummaryInput) (string, error) {
    builder := &defaultPRSummaryBuilder{}
    return builder.Build(ctx, input)
}

// PRSummaryBuilder generates textual summaries for spec PR bodies.
type PRSummaryBuilder interface {
    Build(ctx context.Context, input PRSummaryInput) (string, error)
}

// defaultPRSummaryBuilder is the standard summary generator used by the merge pipeline.
type defaultPRSummaryBuilder struct{}

func (defaultPRSummaryBuilder) Build(ctx context.Context, input PRSummaryInput) (string, error) {
    var b strings.Builder

    if input.SpecName != "" {
        fmt.Fprintf(&b, "## Spec %s\n\n", input.SpecName)
    }

    if excerpt := excerptLines(input.SpecContent, defaultSpecExcerptLines); excerpt != "" {
        b.WriteString("### Spec excerpt\n```\n")
        b.WriteString(excerpt)
        b.WriteString("\n```\n\n")
    }

    if excerpt := excerptLines(input.Diff, defaultDiffExcerptLines); excerpt != "" {
        b.WriteString("### Diff excerpt\n```\n")
        b.WriteString(excerpt)
        b.WriteString("\n```")
    }

    body := strings.TrimSpace(b.String())
    if body == "" {
        return "Spec update ready for review.", nil
    }
    return body, nil
}

func excerptLines(value string, maxLines int) string {
    trimmed := strings.TrimSpace(value)
    if trimmed == "" || maxLines <= 0 {
        return trimmed
    }
    lines := strings.Split(trimmed, "\n")
    if len(lines) <= maxLines {
        return strings.Join(lines, "\n")
    }
    return strings.Join(lines[:maxLines], "\n")
}
