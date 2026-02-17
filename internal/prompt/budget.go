package prompt

import "github.com/danabrams/gromit/internal/learnings"

// ShapeReport describes what trimming was applied to a context.
type ShapeReport struct {
	BeforeChars  int
	AfterChars   int
	TrimActions  []string
	SectionSizes map[string]int
}

// measureContext returns the total character count of trimmable context fields.
func measureContext(ctx *Context) int {
	total := len(ctx.ClaudeMD) + len(ctx.Rules) + len(ctx.Spec)
	for _, l := range ctx.ConfirmedLearnings {
		total += len(l.Content)
	}
	for _, l := range ctx.RecentLearnings {
		total += len(l.Content)
	}
	if ctx.Bead != nil {
		total += len(ctx.Bead.ID) + len(ctx.Bead.Title) + len(ctx.Bead.Description)
	}
	if ctx.ParentBead != nil {
		total += len(ctx.ParentBead.ID) + len(ctx.ParentBead.Title) + len(ctx.ParentBead.Description)
	}
	return total
}

// sectionSizes returns a map of context section names to their character counts.
func sectionSizes(ctx *Context) map[string]int {
	sizes := map[string]int{
		"ClaudeMD": len(ctx.ClaudeMD),
		"Rules":    len(ctx.Rules),
		"Spec":     len(ctx.Spec),
	}
	confirmed := 0
	for _, l := range ctx.ConfirmedLearnings {
		confirmed += len(l.Content)
	}
	sizes["ConfirmedLearnings"] = confirmed

	recent := 0
	for _, l := range ctx.RecentLearnings {
		recent += len(l.Content)
	}
	sizes["RecentLearnings"] = recent

	return sizes
}

// ShapeContextForBudget clones the context and applies deterministic trimming
// to fit within maxChars. Trim order: (1) drop RecentLearnings, (2) drop ClaudeMD,
// (3) cap ConfirmedLearnings to learningCapChars, (4) drop ConfirmedLearnings entirely,
// (5) phase-filter Rules, (6) truncate Spec with head/tail marker.
// Bead identity is never trimmed. Rules and Spec are never fully dropped.
func ShapeContextForBudget(ctx *Context, maxChars int, learningCapChars int, phase string) (*Context, *ShapeReport) {
	shaped := cloneMethodologyContext(ctx)
	beforeChars := measureContext(shaped)

	report := &ShapeReport{
		BeforeChars:  beforeChars,
		SectionSizes: sectionSizes(shaped),
	}

	if beforeChars <= maxChars {
		report.AfterChars = beforeChars
		return shaped, report
	}

	// Step 1: drop RecentLearnings
	if len(shaped.RecentLearnings) > 0 {
		shaped.RecentLearnings = []learnings.Learning{}
		report.TrimActions = append(report.TrimActions, "drop RecentLearnings")
		if measureContext(shaped) <= maxChars {
			return finishReport(shaped, report)
		}
	}

	// Step 2: drop ClaudeMD
	if shaped.ClaudeMD != "" {
		shaped.ClaudeMD = ""
		report.TrimActions = append(report.TrimActions, "drop ClaudeMD")
		if measureContext(shaped) <= maxChars {
			return finishReport(shaped, report)
		}
	}

	// Step 3: cap ConfirmedLearnings to learningCapChars
	if len(shaped.ConfirmedLearnings) > 0 {
		capped := capLearnings(shaped.ConfirmedLearnings, learningCapChars)
		if len(capped) < len(shaped.ConfirmedLearnings) {
			shaped.ConfirmedLearnings = capped
			report.TrimActions = append(report.TrimActions, "cap ConfirmedLearnings")
			if measureContext(shaped) <= maxChars {
				return finishReport(shaped, report)
			}
		}
	}

	return finishReport(shaped, report)
}

// capLearnings keeps the first N learnings that fit within maxChars total content.
func capLearnings(ls []learnings.Learning, maxChars int) []learnings.Learning {
	var result []learnings.Learning
	total := 0
	for _, l := range ls {
		if total+len(l.Content) > maxChars {
			break
		}
		total += len(l.Content)
		result = append(result, l)
	}
	if result == nil {
		result = []learnings.Learning{}
	}
	return result
}

func finishReport(ctx *Context, report *ShapeReport) (*Context, *ShapeReport) {
	report.AfterChars = measureContext(ctx)
	report.SectionSizes = sectionSizes(ctx)
	return ctx, report
}
