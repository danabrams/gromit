package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

const (
	promptPhaseBuild  = "build"
	promptPhaseReview = "review"
)

// Renderer loads and renders prompt templates
type Renderer struct {
	templatesDir           string
	specsDir               string
	claudeMDPath           string
	rulesPath              string
	gromitDir              string
	learningsFile          *learnings.File
	maxLearningChars       int  // Character budget for confirmed learnings; 0 means no cap
	skipBuildLearnings     bool // When true, omit learnings from build prompts (experiment)
	budgetMaxChars         int  // Total prompt budget in chars; 0 means no budget shaping
	budgetLearningCapChars int  // Learning cap used during budget shaping

	// Cache fields - files are immutable during a run, so cache after first load
	claudeMDCache   *string                       // Cached CLAUDE.md content
	rulesCache      *string                       // Cached RULES.md content
	specCache       map[string]string             // Cached spec files by name
	templateCache   map[string]*template.Template // Cached parsed templates by name
	lastDiagnostics *PromptDiagnostics            // Diagnostics from the most recent Render* call

	// Optional callback to resolve sibling-touched packages for prompt enrichment.
	siblingTouchedPackagesResolver SiblingTouchedPackagesResolver
}

// SiblingTouchedPackagesResolver resolves touched packages from sibling beads.
type SiblingTouchedPackagesResolver func(current *bead.Bead, parent *bead.Bead) ([]string, error)

// NewRenderer creates a new prompt renderer
func NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir string) (*Renderer, error) {
	lf, err := learnings.NewFile(gromitDir)
	if err != nil {
		return nil, err
	}
	lf.Load() // Ignore error - learnings are optional

	return &Renderer{
		templatesDir:  templatesDir,
		specsDir:      specsDir,
		claudeMDPath:  claudeMDPath,
		rulesPath:     filepath.Join(gromitDir, "RULES.md"),
		gromitDir:     gromitDir,
		learningsFile: lf,
		specCache:     make(map[string]string),
	}, nil
}

// GetLearningsFile returns the learnings file for external use
func (r *Renderer) GetLearningsFile() *learnings.File {
	if r == nil {
		return nil
	}
	return r.learningsFile
}

// GetSpecsDir returns the specs directory path
func (r *Renderer) GetSpecsDir() string {
	if r == nil {
		return ""
	}
	return r.specsDir
}

// GetGromitDir returns the gromit directory path
func (r *Renderer) GetGromitDir() string {
	if r == nil {
		return ""
	}
	return r.gromitDir
}

// SetMaxLearningChars sets the character budget for confirmed learnings.
// Zero means no cap (backward compatible).
func (r *Renderer) SetMaxLearningChars(maxChars int) {
	if r == nil {
		return
	}
	r.maxLearningChars = maxChars
}

// SetSkipBuildLearnings controls whether learnings are omitted from build prompts.
func (r *Renderer) SetSkipBuildLearnings(skip bool) {
	if r == nil {
		return
	}
	r.skipBuildLearnings = skip
}

// SetBudgetConfig sets the prompt budget shaping configuration.
// When maxChars > 0, qualifying render methods will call ShapeContextForBudget
// before rendering to trim context that exceeds the budget.
func (r *Renderer) SetBudgetConfig(maxChars, learningCapChars int) {
	if r == nil {
		return
	}
	r.budgetMaxChars = maxChars
	r.budgetLearningCapChars = learningCapChars
}

// SetSiblingTouchedPackagesResolver configures optional sibling context enrichment.
func (r *Renderer) SetSiblingTouchedPackagesResolver(resolver SiblingTouchedPackagesResolver) {
	if r == nil {
		return
	}
	r.siblingTouchedPackagesResolver = resolver
}

// LastDiagnostics returns diagnostics from the most recent Render* call.
func (r *Renderer) LastDiagnostics() *PromptDiagnostics {
	if r == nil {
		return nil
	}
	return r.lastDiagnostics
}

// LoadSpec loads a spec file by name
func (r *Renderer) LoadSpec(name string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}
	if err := ValidateSpecName(name); err != nil {
		return "", err
	}

	// Lazy initialize cache if needed (tests may create Renderer directly)
	if r.specCache == nil {
		r.specCache = make(map[string]string)
	}

	// Return cached content if already loaded
	if content, ok := r.specCache[name]; ok {
		return content, nil
	}

	path := filepath.Join(r.specsDir, name+".md")

	// Belt-and-suspenders: verify resolved path stays within specsDir
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving spec path: %w", err)
	}
	absSpecsDir, err := filepath.Abs(r.specsDir)
	if err != nil {
		return "", fmt.Errorf("resolving specs dir: %w", err)
	}
	if !strings.HasPrefix(absPath, absSpecsDir+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid spec name %q: resolves outside specs directory", name)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Spec doesn't exist - not an error
		}
		return "", fmt.Errorf("reading spec %s: %w", name, err)
	}
	contentStr := string(content)
	r.specCache[name] = contentStr
	return contentStr, nil
}

// LoadClaudeMD loads the project's CLAUDE.md
func (r *Renderer) LoadClaudeMD() (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}
	// Return cached content if already loaded
	if r.claudeMDCache != nil {
		return *r.claudeMDCache, nil
	}
	content, err := os.ReadFile(r.claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading CLAUDE.md: %w", err)
	}
	contentStr := string(content)
	r.claudeMDCache = &contentStr
	return contentStr, nil
}

// BuildContext builds a complete prompt context for a bead
func (r *Renderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*Context, error) {
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

	// Load CLAUDE.md
	claudeMD, err := r.LoadClaudeMD()
	if err != nil {
		return nil, err
	}
	ctx.ClaudeMD = claudeMD

	// Load RULES.md
	rules, err := r.LoadRules()
	if err != nil {
		return nil, err
	}
	ctx.Rules = rules

	// Load learnings (skipped during build-learnings-pruning experiment)
	if r.learningsFile != nil && !r.skipBuildLearnings {
		ctx.ConfirmedLearnings = r.learningsFile.GetConfirmedFiltered(learnings.FilterOptions{
			MaxChars: r.maxLearningChars,
		})
		ctx.RecentLearnings = r.learningsFile.GetRecent(24) // Last 24 hours
	}

	// Get working directory
	ctx.WorkDir, _ = os.Getwd()

	// Find and load spec (check bead first, then parent).
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

// LoadRules loads the RULES.md file
func (r *Renderer) LoadRules() (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}
	// Return cached content if already loaded
	if r.rulesCache != nil {
		return *r.rulesCache, nil
	}
	content, err := os.ReadFile(r.rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading RULES.md: %w", err)
	}
	contentStr := string(content)
	r.rulesCache = &contentStr
	return contentStr, nil
}

// LoadRulesForPhase loads RULES.md and returns only sections matching the given phase.
// Sections with <!-- phases: build, review --> annotations are included only when the
// requested phase appears in their phase list. Sections without annotations are included
// in all phases. Phase annotation comments are stripped from the output.
func (r *Renderer) LoadRulesForPhase(phase string) (string, error) {
	content, err := r.LoadRules()
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}

	return filterRulesByPhase(content, phase), nil
}

// shapeBuildContext applies budget shaping to a Context before rendering.
// Returns the original context if budget is zero or context is under budget.
func (r *Renderer) shapeBuildContext(ctx *Context, phase string) (*Context, *ShapeReport) {
	if r == nil || r.budgetMaxChars <= 0 || ctx == nil {
		return ctx, nil
	}
	shaped, report := ShapeContextForBudget(ctx, r.budgetMaxChars, r.budgetLearningCapChars, phase)
	logBudgetTrim(report)
	return shaped, report
}

// shapeReviewContext applies budget shaping to a ReviewContext before rendering.
func (r *Renderer) shapeReviewContext(ctx *ReviewContext) (*ReviewContext, *ShapeReport) {
	if r == nil || r.budgetMaxChars <= 0 || ctx == nil {
		return ctx, nil
	}
	shaped, report := ShapeReviewContextForBudget(ctx, r.budgetMaxChars, promptPhaseReview)
	logBudgetTrim(report)
	return shaped, report
}

// shapeThoroughReviewContext applies budget shaping to a ThoroughReviewContext before rendering.
func (r *Renderer) shapeThoroughReviewContext(ctx *ThoroughReviewContext) (*ThoroughReviewContext, *ShapeReport) {
	if r == nil || r.budgetMaxChars <= 0 || ctx == nil {
		return ctx, nil
	}
	shaped, report := ShapeThoroughReviewContextForBudget(ctx, r.budgetMaxChars, promptPhaseReview)
	logBudgetTrim(report)
	return shaped, report
}

func logBudgetTrim(report *ShapeReport) {
	if report == nil || len(report.TrimActions) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "Prompt budget: %d -> %d chars (trimmed: %s)\n",
		report.BeforeChars, report.AfterChars, strings.Join(report.TrimActions, ", "))
}

func (r *Renderer) render(templateName string, ctx any) (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}

	// Use cached template if available; otherwise load from disk and cache.
	// Templates are frozen at first access so mid-run file changes (e.g. a
	// bead modifying its own templates) cannot break subsequent iterations.
	tmpl, ok := r.templateCache[templateName]
	if !ok {
		path := filepath.Join(r.templatesDir, templateName)
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading template %s: %w", templateName, err)
		}

		tmpl, err = template.New(templateName).Option("missingkey=zero").Funcs(templateFuncs()).Parse(string(content))
		if err != nil {
			return "", fmt.Errorf("parsing template %s: %w", templateName, err)
		}

		if r.templateCache == nil {
			r.templateCache = make(map[string]*template.Template)
		}
		r.templateCache[templateName] = tmpl
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("executing template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"join":     strings.Join,
		"contains": strings.Contains,
		"hasLabel": func(labels []string, target string) bool {
			return bead.HasLabel(labels, target)
		},
		"indent": func(spaces int, s string) string {
			pad := strings.Repeat(" ", spaces)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if line != "" {
					lines[i] = pad + line
				}
			}
			return strings.Join(lines, "\n")
		},
		"formatLearnings": func(ls []learnings.Learning) string {
			if len(ls) == 0 {
				return "*None*"
			}
			var sb strings.Builder
			for _, l := range ls {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", l.Category, l.Content))
			}
			return sb.String()
		},
	}
}
