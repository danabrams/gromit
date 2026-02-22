package prompt

import (
	"fmt"
	"os"
	"sort"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

// BuildContext builds a complete prompt context for a bead.
func (r *Renderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string, phase string) (*Context, error) {
	if r == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	if b == nil {
		return nil, fmt.Errorf("bead is nil")
	}
	ctx := &Context{
		Bead:       b,
		ParentBead: parent,
		Iteration:  iteration,
		Model:      model,
	}

	claudeMD, err := r.LoadClaudeMD()
	if err != nil {
		return nil, err
	}
	ctx.ClaudeMD = claudeMD

	var rules string
	if phase == "" {
		rules, err = r.LoadRules()
	} else {
		rules, err = r.LoadRulesForPhase(phase)
	}
	if err != nil {
		return nil, err
	}
	ctx.Rules = rules

	if r.learningsFile != nil && !r.skipBuildLearnings {
		ctx.ConfirmedLearnings = r.learningsFile.GetConfirmedFiltered(learnings.FilterOptions{
			MaxChars: r.maxLearningChars,
		})
		ctx.RecentLearnings = r.learningsFile.GetRecent(24)
	}

	ctx.WorkDir, _ = os.Getwd()

	specName := specLabelFromCurrentOrParent(b, parent)
	if specName != "" {
		spec, err := r.LoadSpec(specName)
		if err != nil {
			return nil, err
		}
		ctx.Spec = spec
		ctx.SpecName = specName
	}

	r.enrichSiblingTouchedPackages(ctx, b, parent)
	r.applyScopedClaudeContext(ctx)
	ApplyPhaseProfile(ctx, phase)

	ctx.normalizeNilFields()
	return ctx, nil
}

// specLabelFromCurrentOrParent resolves the spec label from the current bead first,
// then falls back to the parent bead.
func specLabelFromCurrentOrParent(current, parent *bead.Bead) string {
	if current == nil {
		return ""
	}
	specName := bead.FindSpecLabel(current.Labels)
	if specName == "" && parent != nil {
		specName = bead.FindSpecLabel(parent.Labels)
	}
	return specName
}

// enrichSiblingTouchedPackages applies optional sibling package enrichment.
// Resolver failures intentionally degrade to no enrichment.
func (r *Renderer) enrichSiblingTouchedPackages(ctx *Context, current, parent *bead.Bead) {
	if r == nil || ctx == nil || r.siblingTouchedPackagesResolver == nil {
		return
	}
	siblingTouched, err := r.siblingTouchedPackagesResolver(current, parent)
	if err == nil && siblingTouched != nil {
		ctx.SiblingTouchedPackages = siblingTouched
	}
}

// applyScopedClaudeContext scopes CLAUDE architecture context when package discovery finds targets.
// Fallback remains the full CLAUDE.md content when no scoped packages are discovered.
func (r *Renderer) applyScopedClaudeContext(ctx *Context) {
	if r == nil || ctx == nil {
		return
	}

	scopedPaths := collectScopedPackagePaths(ctx)
	if len(scopedPaths) == 0 {
		return
	}

	sections := parseClaudeSections(ctx.ClaudeMD)
	entries := resolveScopedArchitectureEntries(scopedPaths, sections.ArchitectureBody, ctx.WorkDir)
	ctx.ClaudeMD = renderScopedClaudeContent(ctx.ClaudeMD, entries)
}

// collectScopedPackagePaths merges Layer 1 extraction (spec/task text) and Layer 2
// enrichment (sibling-touched packages), then normalizes and sorts the result.
func collectScopedPackagePaths(ctx *Context) []string {
	if ctx == nil {
		return []string{}
	}

	layer1Paths := extractScopedPackagePathsFromText(
		ctx.Spec,
		beadDescription(ctx.Bead),
		beadDescription(ctx.ParentBead),
	)

	return mergeScopedPackagePaths(layer1Paths, ctx.SiblingTouchedPackages)
}

func mergeScopedPackagePaths(pathGroups ...[]string) []string {
	if len(pathGroups) == 0 {
		return []string{}
	}

	unique := make(map[string]struct{})
	for _, group := range pathGroups {
		for _, candidate := range group {
			normalized := normalizeScopedPath(candidate)
			if normalized == "" {
				continue
			}
			unique[normalized] = struct{}{}
		}
	}

	merged := make([]string, 0, len(unique))
	for path := range unique {
		merged = append(merged, path)
	}
	if len(merged) == 0 {
		return []string{}
	}
	sort.Strings(merged)
	return merged
}

func beadDescription(b *bead.Bead) string {
	if b == nil {
		return ""
	}
	return b.Description
}
