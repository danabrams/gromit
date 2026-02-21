package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

const (
	promptPhaseBuild  = "build"
	promptPhaseReview = "review"

	trimDropATDDRules    = "drop ATDD Rules"
	trimReplaceATDDRules = "replace ATDD Rules"
	trimDropATDDSpec     = "drop ATDD Spec"
	trimCapATDDLearnings = "cap ATDD ConfirmedLearnings"
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
	atddPromptConfig               *ATDDPromptConfig
}

// ATDDPromptConfig controls context shaping for ATDD-specific builds.
type ATDDPromptConfig struct {
	IncludeRules              bool
	IncludeSpec               bool
	IncludeClaudeMD           bool
	MaxChars                  int
	MaxConfirmedLearningChars int
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

// SetATDDPromptConfig stores ATDD-specific prompt shaping config.
func (r *Renderer) SetATDDPromptConfig(cfg ATDDPromptConfig) {
	if r == nil {
		return
	}
	r.atddPromptConfig = &ATDDPromptConfig{
		IncludeRules:              cfg.IncludeRules,
		IncludeSpec:               cfg.IncludeSpec,
		IncludeClaudeMD:           cfg.IncludeClaudeMD,
		MaxChars:                  cfg.MaxChars,
		MaxConfirmedLearningChars: cfg.MaxConfirmedLearningChars,
	}
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
