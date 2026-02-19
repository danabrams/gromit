package prompt

import (
	"strings"
	"unicode/utf8"

	"github.com/danabrams/gromit/internal/learnings"
)

const (
	trimDropRecentLearnings    = "drop RecentLearnings"
	trimDropClaudeMD           = "drop ClaudeMD"
	trimCapConfirmedLearnings  = "cap ConfirmedLearnings"
	trimDropConfirmedLearnings = "drop ConfirmedLearnings"
	trimPhaseFilterRules       = "phase-filter Rules"
	trimTruncateSpec           = "truncate Spec"
	trimTruncateDiff           = "truncate Diff"
	trimCapRetroLearnings      = "cap RetroLearnings"
	trimDropRetroLearnings     = "drop RetroLearnings"
	trimTruncateRetroRules     = "truncate RetroRules"
	retroTruncationMarker      = "[...truncated...]"
	minHeadTailKeepChars       = 10
)

// truncateUTF8 returns s truncated to at most maxBytes bytes, never splitting a
// multi-byte UTF-8 rune. It walks backward from maxBytes to find the last valid
// rune start boundary.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	// Walk backward from maxBytes to find a valid rune start.
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}
	return s[:maxBytes]
}

func measureRetroContext(rules, learnings string) int {
	return len(rules) + len(learnings)
}

// ShapeReport describes what trimming was applied to a context.
type ShapeReport struct {
	BeforeChars     int
	AfterChars      int
	PreShapeTokens  int
	PostShapeTokens int
	TrimActions     []string
	SectionSizes    map[string]int
}

// ShapeRetroForBudget trims retro rules/learnings context to fit within maxChars.
func ShapeRetroForBudget(rules, learnings string, maxChars int) (string, string, *ShapeReport) {
	shapedRules := rules
	shapedLearnings := learnings

	beforeChars := measureRetroContext(shapedRules, shapedLearnings)
	preShapeTokens := measureRetroContextTokens(shapedRules, shapedLearnings)
	report := &ShapeReport{
		BeforeChars:     beforeChars,
		AfterChars:      beforeChars,
		PreShapeTokens:  preShapeTokens,
		PostShapeTokens: preShapeTokens,
		SectionSizes: map[string]int{
			"Rules":     len(shapedRules),
			"Learnings": len(shapedLearnings),
		},
	}

	if maxChars <= 0 || beforeChars <= maxChars {
		return shapedRules, shapedLearnings, report
	}

	halfBudget := maxChars / 2

	if len(shapedLearnings) > halfBudget {
		shapedLearnings = truncateUTF8(shapedLearnings, halfBudget)
		report.TrimActions = append(report.TrimActions, trimCapRetroLearnings)
		if measureRetroContext(shapedRules, shapedLearnings) <= maxChars {
			return finishRetroReport(shapedRules, shapedLearnings, report)
		}
	}

	if shapedLearnings != "" {
		shapedLearnings = ""
		report.TrimActions = append(report.TrimActions, trimDropRetroLearnings)
		if measureRetroContext(shapedRules, shapedLearnings) <= maxChars {
			return finishRetroReport(shapedRules, shapedLearnings, report)
		}
	}

	maxRulesChars := maxChars - len(shapedLearnings)
	if truncatedRules, changed := truncateRetroRulesForBudget(shapedRules, maxRulesChars); changed {
		shapedRules = truncatedRules
		report.TrimActions = append(report.TrimActions, trimTruncateRetroRules)
	}

	return finishRetroReport(shapedRules, shapedLearnings, report)
}

func finishRetroReport(rules, learnings string, report *ShapeReport) (string, string, *ShapeReport) {
	report.AfterChars = measureRetroContext(rules, learnings)
	report.PostShapeTokens = measureRetroContextTokens(rules, learnings)
	report.SectionSizes["Rules"] = len(rules)
	report.SectionSizes["Learnings"] = len(learnings)
	return rules, learnings, report
}

func truncateRetroRulesForBudget(rules string, maxChars int) (string, bool) {
	if len(rules) <= maxChars {
		return rules, false
	}
	if maxChars <= 0 {
		return "", true
	}
	if maxChars <= len(retroTruncationMarker) {
		return retroTruncationMarker[:maxChars], true
	}

	headLen := maxChars - len(retroTruncationMarker)
	return truncateUTF8(rules, headLen) + retroTruncationMarker, true
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

func measureRetroContextTokens(rules, learnings string) int {
	return EstimateTokens(rules) + EstimateTokens(learnings)
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
	preShapeTokens := measureContextTokens(shaped)

	report := &ShapeReport{
		BeforeChars:     beforeChars,
		PreShapeTokens:  preShapeTokens,
		PostShapeTokens: preShapeTokens,
		SectionSizes:    sectionSizes(shaped),
	}

	if beforeChars <= maxChars {
		report.AfterChars = beforeChars
		return shaped, report
	}

	// Step 1: drop RecentLearnings
	if len(shaped.RecentLearnings) > 0 {
		shaped.RecentLearnings = []learnings.Learning{}
		report.TrimActions = append(report.TrimActions, trimDropRecentLearnings)
		if measureContext(shaped) <= maxChars {
			return finishReport(shaped, report)
		}
	}

	// Step 2: drop ClaudeMD
	if shaped.ClaudeMD != "" {
		shaped.ClaudeMD = ""
		report.TrimActions = append(report.TrimActions, trimDropClaudeMD)
		if measureContext(shaped) <= maxChars {
			return finishReport(shaped, report)
		}
	}

	// Step 3: cap ConfirmedLearnings to learningCapChars
	if len(shaped.ConfirmedLearnings) > 0 {
		capped := capLearnings(shaped.ConfirmedLearnings, learningCapChars)
		if len(capped) > 0 && len(capped) < len(shaped.ConfirmedLearnings) {
			shaped.ConfirmedLearnings = capped
			report.TrimActions = append(report.TrimActions, trimCapConfirmedLearnings)
			if measureContext(shaped) <= maxChars {
				return finishReport(shaped, report)
			}
		}
	}

	// Step 4: drop ConfirmedLearnings entirely
	if len(shaped.ConfirmedLearnings) > 0 {
		shaped.ConfirmedLearnings = []learnings.Learning{}
		report.TrimActions = append(report.TrimActions, trimDropConfirmedLearnings)
		if measureContext(shaped) <= maxChars {
			return finishReport(shaped, report)
		}
	}

	// Step 5: phase-filter Rules
	if filtered, ok := maybePhaseFilterRules(shaped.Rules, phase, true); ok {
		shaped.Rules = filtered
		report.TrimActions = append(report.TrimActions, trimPhaseFilterRules)
		if measureContext(shaped) <= maxChars {
			return finishReport(shaped, report)
		}
	}

	// Step 6: truncate Spec with head/tail + marker
	if shaped.Spec != "" {
		if truncated, ok := truncateFieldForBudget(shaped.Spec, measureContext(shaped), maxChars); ok {
			shaped.Spec = truncated
			report.TrimActions = append(report.TrimActions, trimTruncateSpec)
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

const truncationMarker = "...[truncated]..."

func maybePhaseFilterRules(rules, phase string, requireNonEmpty bool) (string, bool) {
	if phase == "" || rules == "" {
		return rules, false
	}

	filtered := filterRulesByPhase(rules, phase)
	if len(filtered) >= len(rules) {
		return rules, false
	}
	if requireNonEmpty && strings.TrimSpace(filtered) == "" {
		return rules, false
	}

	return filtered, true
}

func truncateFieldForBudget(spec string, currentChars, maxChars int) (string, bool) {
	excess := currentChars - maxChars
	targetLen := len(spec) - excess
	if targetLen <= 0 {
		targetLen = len(truncationMarker)
	}
	if targetLen >= len(spec) {
		return spec, false
	}
	return truncateWithMarker(spec, targetLen), true
}

// truncateWithMarker keeps the head and tail of s, inserting a truncation marker.
// targetLen is the approximate target length for the result.
func truncateWithMarker(s string, targetLen int) string {
	markerLen := len(truncationMarker)
	if targetLen <= markerLen {
		// Keep a minimal head and tail so both ends remain present.
		headKeep := min(minHeadTailKeepChars, len(s))
		tailKeep := min(minHeadTailKeepChars, len(s)-headKeep)
		if tailKeep == 0 {
			return s[:headKeep] + truncationMarker
		}
		return s[:headKeep] + truncationMarker + s[len(s)-tailKeep:]
	}
	available := targetLen - markerLen
	headLen := available * 2 / 3
	tailLen := available - headLen
	if headLen <= 0 || tailLen <= 0 {
		return s[:min(len(s), targetLen)] + truncationMarker
	}
	return s[:headLen] + truncationMarker + s[len(s)-tailLen:]
}

func finishReport(ctx *Context, report *ShapeReport) (*Context, *ShapeReport) {
	report.AfterChars = measureContext(ctx)
	report.PostShapeTokens = measureContextTokens(ctx)
	report.SectionSizes = sectionSizes(ctx)
	return ctx, report
}

// measureReviewContext returns the total character count of ReviewContext fields.
func measureReviewContext(ctx *ReviewContext) int {
	total := len(ctx.ClaudeMD) + len(ctx.Rules) + len(ctx.Spec) + len(ctx.Diff)
	if ctx.Bead != nil {
		total += len(ctx.Bead.ID) + len(ctx.Bead.Title) + len(ctx.Bead.Description)
	}
	if ctx.ParentBead != nil {
		total += len(ctx.ParentBead.ID) + len(ctx.ParentBead.Title) + len(ctx.ParentBead.Description)
	}
	return total
}

func measureContextTokens(ctx *Context) int {
	total := EstimateTokens(ctx.ClaudeMD) + EstimateTokens(ctx.Rules) + EstimateTokens(ctx.Spec)
	for _, l := range ctx.ConfirmedLearnings {
		total += EstimateTokens(l.Content)
	}
	for _, l := range ctx.RecentLearnings {
		total += EstimateTokens(l.Content)
	}
	if ctx.Bead != nil {
		total += EstimateTokens(ctx.Bead.ID) + EstimateTokens(ctx.Bead.Title) + EstimateTokens(ctx.Bead.Description)
	}
	if ctx.ParentBead != nil {
		total += EstimateTokens(ctx.ParentBead.ID) + EstimateTokens(ctx.ParentBead.Title) + EstimateTokens(ctx.ParentBead.Description)
	}
	return total
}

// reviewSectionSizes returns section sizes for a ReviewContext.
func reviewSectionSizes(ctx *ReviewContext) map[string]int {
	return map[string]int{
		"ClaudeMD": len(ctx.ClaudeMD),
		"Rules":    len(ctx.Rules),
		"Spec":     len(ctx.Spec),
		"Diff":     len(ctx.Diff),
	}
}

// cloneReviewContext deep-clones a ReviewContext.
func cloneReviewContext(ctx *ReviewContext) *ReviewContext {
	cloned := *ctx
	cloned.Bead = cloneBead(ctx.Bead)
	cloned.ParentBead = cloneBead(ctx.ParentBead)
	cloned.ValidationCommands = append([]string{}, ctx.ValidationCommands...)
	return &cloned
}

// ShapeReviewContextForBudget trims a ReviewContext to fit within maxChars.
// Trim order: (1) drop ClaudeMD, (2) phase-filter Rules, (3) truncate Spec.
// Diff, bead identity, and Rules are never fully dropped.
func ShapeReviewContextForBudget(ctx *ReviewContext, maxChars int, phase string) (*ReviewContext, *ShapeReport) {
	shaped := cloneReviewContext(ctx)
	beforeChars := measureReviewContext(shaped)
	preShapeTokens := measureReviewContextTokens(shaped)

	report := &ShapeReport{
		BeforeChars:     beforeChars,
		PreShapeTokens:  preShapeTokens,
		PostShapeTokens: preShapeTokens,
		SectionSizes:    reviewSectionSizes(shaped),
	}

	if beforeChars <= maxChars {
		report.AfterChars = beforeChars
		return shaped, report
	}

	// Step 1: drop ClaudeMD
	if shaped.ClaudeMD != "" {
		shaped.ClaudeMD = ""
		report.TrimActions = append(report.TrimActions, trimDropClaudeMD)
		if measureReviewContext(shaped) <= maxChars {
			return finishReviewReport(shaped, report)
		}
	}

	// Step 2: phase-filter Rules
	if filtered, ok := maybePhaseFilterRules(shaped.Rules, phase, true); ok {
		shaped.Rules = filtered
		report.TrimActions = append(report.TrimActions, trimPhaseFilterRules)
		if measureReviewContext(shaped) <= maxChars {
			return finishReviewReport(shaped, report)
		}
	}

	// Step 3: truncate Spec
	if shaped.Spec != "" {
		if truncated, ok := truncateFieldForBudget(shaped.Spec, measureReviewContext(shaped), maxChars); ok {
			shaped.Spec = truncated
			report.TrimActions = append(report.TrimActions, trimTruncateSpec)
		}
	}

	return finishReviewReport(shaped, report)
}

func finishReviewReport(ctx *ReviewContext, report *ShapeReport) (*ReviewContext, *ShapeReport) {
	report.AfterChars = measureReviewContext(ctx)
	report.PostShapeTokens = measureReviewContextTokens(ctx)
	report.SectionSizes = reviewSectionSizes(ctx)
	return ctx, report
}

// measureThoroughReviewContext returns the total character count of ThoroughReviewContext fields.
func measureThoroughReviewContext(ctx *ThoroughReviewContext) int {
	total := len(ctx.ClaudeMD) + len(ctx.Rules) + len(ctx.Diff)
	for _, b := range ctx.CompletedBeads {
		total += len(b.ID) + len(b.Title) + len(b.Description)
	}
	return total
}

func measureReviewContextTokens(ctx *ReviewContext) int {
	total := EstimateTokens(ctx.ClaudeMD) + EstimateTokens(ctx.Rules) + EstimateTokens(ctx.Spec) + EstimateTokens(ctx.Diff)
	if ctx.Bead != nil {
		total += EstimateTokens(ctx.Bead.ID) + EstimateTokens(ctx.Bead.Title) + EstimateTokens(ctx.Bead.Description)
	}
	if ctx.ParentBead != nil {
		total += EstimateTokens(ctx.ParentBead.ID) + EstimateTokens(ctx.ParentBead.Title) + EstimateTokens(ctx.ParentBead.Description)
	}
	return total
}

// thoroughReviewSectionSizes returns section sizes for a ThoroughReviewContext.
func thoroughReviewSectionSizes(ctx *ThoroughReviewContext) map[string]int {
	beads := 0
	for _, b := range ctx.CompletedBeads {
		beads += len(b.ID) + len(b.Title) + len(b.Description)
	}
	return map[string]int{
		"ClaudeMD":       len(ctx.ClaudeMD),
		"Rules":          len(ctx.Rules),
		"Diff":           len(ctx.Diff),
		"CompletedBeads": beads,
	}
}

// cloneThoroughReviewContext deep-clones a ThoroughReviewContext.
func cloneThoroughReviewContext(ctx *ThoroughReviewContext) *ThoroughReviewContext {
	cloned := *ctx
	cloned.CompletedBeads = append([]CompletedBeadSummary{}, ctx.CompletedBeads...)
	return &cloned
}

// ShapeThoroughReviewContextForBudget trims a ThoroughReviewContext to fit within maxChars.
// Trim order: (1) drop ClaudeMD, (2) phase-filter Rules, (3) truncate Diff.
// CompletedBeads and Rules are never fully dropped.
func ShapeThoroughReviewContextForBudget(ctx *ThoroughReviewContext, maxChars int, phase string) (*ThoroughReviewContext, *ShapeReport) {
	shaped := cloneThoroughReviewContext(ctx)
	beforeChars := measureThoroughReviewContext(shaped)
	preShapeTokens := measureThoroughReviewContextTokens(shaped)

	report := &ShapeReport{
		BeforeChars:     beforeChars,
		PreShapeTokens:  preShapeTokens,
		PostShapeTokens: preShapeTokens,
		SectionSizes:    thoroughReviewSectionSizes(shaped),
	}

	if beforeChars <= maxChars {
		report.AfterChars = beforeChars
		return shaped, report
	}

	// Step 1: drop ClaudeMD
	if shaped.ClaudeMD != "" {
		shaped.ClaudeMD = ""
		report.TrimActions = append(report.TrimActions, trimDropClaudeMD)
		if measureThoroughReviewContext(shaped) <= maxChars {
			return finishThoroughReviewReport(shaped, report)
		}
	}

	// Step 2: phase-filter Rules
	if filtered, ok := maybePhaseFilterRules(shaped.Rules, phase, true); ok {
		shaped.Rules = filtered
		report.TrimActions = append(report.TrimActions, trimPhaseFilterRules)
		if measureThoroughReviewContext(shaped) <= maxChars {
			return finishThoroughReviewReport(shaped, report)
		}
	}

	// Step 3: truncate Diff
	if shaped.Diff != "" {
		if truncated, ok := truncateFieldForBudget(shaped.Diff, measureThoroughReviewContext(shaped), maxChars); ok {
			shaped.Diff = truncated
			report.TrimActions = append(report.TrimActions, trimTruncateDiff)
		}
	}

	return finishThoroughReviewReport(shaped, report)
}

func finishThoroughReviewReport(ctx *ThoroughReviewContext, report *ShapeReport) (*ThoroughReviewContext, *ShapeReport) {
	report.AfterChars = measureThoroughReviewContext(ctx)
	report.PostShapeTokens = measureThoroughReviewContextTokens(ctx)
	report.SectionSizes = thoroughReviewSectionSizes(ctx)
	return ctx, report
}

func measureThoroughReviewContextTokens(ctx *ThoroughReviewContext) int {
	total := EstimateTokens(ctx.ClaudeMD) + EstimateTokens(ctx.Rules) + EstimateTokens(ctx.Diff)
	for _, b := range ctx.CompletedBeads {
		total += EstimateTokens(b.ID) + EstimateTokens(b.Title) + EstimateTokens(b.Description)
	}
	return total
}
