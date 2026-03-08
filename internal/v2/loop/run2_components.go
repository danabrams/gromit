package loop

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/event"
	v2remediation "github.com/danabrams/gromit/internal/v2/remediation"
	"github.com/danabrams/gromit/internal/v2/routing"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	acceptstage "github.com/danabrams/gromit/internal/v2/stage/accept"
	buildstage "github.com/danabrams/gromit/internal/v2/stage/build"
	decomposestage "github.com/danabrams/gromit/internal/v2/stage/decompose"
	epiloguestage "github.com/danabrams/gromit/internal/v2/stage/epilogue"
	gatestage "github.com/danabrams/gromit/internal/v2/stage/gate"
	planstage "github.com/danabrams/gromit/internal/v2/stage/plan"
	present "github.com/danabrams/gromit/internal/v2/stage/present"
	reviewstage "github.com/danabrams/gromit/internal/v2/stage/review"
	triagestage "github.com/danabrams/gromit/internal/v2/stage/triage"
	stagevalidate "github.com/danabrams/gromit/internal/v2/stage/validate"
)

// Compile-time check: PlanLLMAdapter must satisfy adapter.LLMAdapter.
var _ adapter.LLMAdapter = (*llm.PlanLLMAdapter)(nil)

// Run2LoopEmitter exposes the subset of event emitter behavior needed by Run2.
type Run2LoopEmitter interface {
	Close()
}

// Run2LoopComponents groups the stages and bead loop used by the Run2 command.
type Run2LoopComponents struct {
	PlanStage             stagepkg.Stage
	PresentStage          stagepkg.Stage
	PresentSummaryContext *present.SummaryContext
	DecomposeStage        stagepkg.Stage
	BeadLoop              *BeadLoop
	AcceptStage           stagepkg.Stage
	RemediationRunner     remediationRunner
	Emitter               Run2LoopEmitter
	StageCommitter        StageCommitter
	TypedEmitter          *event.Emitter
}

// NewRun2LoopComponents builds the stages and bead loop that power the Run2 command.
func NewRun2LoopComponents(cfg *config.Config, adapters adapter.AdapterSet, legacyEmitter *events.Emitter, output io.Writer, router *routing.Router, phaseModels map[string]string) (*Run2LoopComponents, error) {
	typedEmitter := event.NewEmitter()
	cleanup := func() {
		typedEmitter.Close()
	}

	// Load prompt contexts and fragments from project root
	projectContext, err := loadProjectContext(cfg.ProjectRoot)
	if err != nil {
		cleanup()
		return nil, err
	}

	baseInstructions, err := loadBaseInstructions(cfg.ProjectRoot)
	if err != nil {
		cleanup()
		return nil, err
	}

	fragments, err := loadMethodologyFragments(cfg.ProjectRoot)
	if err != nil {
		cleanup()
		return nil, err
	}

	// Load stage-specific fragments
	reviewFragment, err := loadFragment(cfg.ProjectRoot, "review_v2.md")
	if err != nil {
		cleanup()
		return nil, err
	}
	acceptFragment, err := loadFragment(cfg.ProjectRoot, "accept_v2.md")
	if err != nil {
		cleanup()
		return nil, err
	}
	triageFragment, err := loadFragment(cfg.ProjectRoot, "triage_v2.md")
	if err != nil {
		cleanup()
		return nil, err
	}
	planFragment, err := loadFragment(cfg.ProjectRoot, "plan_v2.md")
	if err != nil {
		cleanup()
		return nil, err
	}

	planStage, err := planstage.New(cfg, adapters.LLM, baseInstructions, projectContext, planFragment)
	if err != nil {
		cleanup()
		return nil, err
	}
	summaryCtx := &present.SummaryContext{}
	presentStage, err := present.New(cfg, adapters.Presenter, summaryCtx, present.WithSquashGit(adapters.Git))
	if err != nil {
		cleanup()
		return nil, err
	}

	decomposeTemplate, err := loadFragment(cfg.ProjectRoot, "decompose_v2.md")
	if err != nil {
		cleanup()
		return nil, err
	}

	var decomposeOpts []decomposestage.Option
	if decomposeTemplate != "" {
		decomposeOpts = append(decomposeOpts, decomposestage.WithPromptTemplate(decomposeTemplate))
	}

	decomposeStage, err := decomposestage.New(cfg, adapters.LLM, adapters.TaskTracker, decomposeOpts...)
	if err != nil {
		cleanup()
		return nil, err
	}

	gateStage, err := gatestage.New(cfg, adapters.TaskTracker)
	if err != nil {
		cleanup()
		return nil, err
	}

	buildStage, err := buildstage.New(cfg, adapters.LLM, baseInstructions, projectContext, fragments, output)
	if err != nil {
		cleanup()
		return nil, err
	}

	validateStage, err := stagevalidate.New(cfg, NewCommandValidationRunner("."))
	if err != nil {
		cleanup()
		return nil, err
	}

	reviewStage, err := reviewstage.New(cfg, adapters.Git, adapters.LLM, adapters.TaskTracker, baseInstructions, projectContext, reviewFragment)
	if err != nil {
		cleanup()
		return nil, err
	}
	reviewStage = reviewStage.WithEmitter(legacyEmitter)

	triageStage, err := triagestage.New(cfg, adapters.LLM, baseInstructions, projectContext, triageFragment)
	if err != nil {
		cleanup()
		return nil, err
	}

	epilogueStage, err := epiloguestage.New(cfg, adapters.TaskTracker)
	if err != nil {
		cleanup()
		return nil, err
	}

	sc := &gitadapter.StageCommitter{Git: adapters.Git}

	beadLoop, err := NewBeadLoop(BeadLoopConfig{
		Gate:           gateStage,
		Build:          buildStage,
		Validate:       validateStage,
		Review:         reviewStage,
		Epilogue:       epilogueStage,
		Triage:         triageStage,
		Decompose:      decomposeStage,
		Emitter:        typedEmitter,
		LegacyEmitter:  legacyEmitter,
		Git:            adapters.Git,
		StageCommitter: sc,
		Router:         router,
		PhaseModels:    phaseModels,
	})
	if err != nil {
		cleanup()
		return nil, err
	}

	acceptStage, err := acceptstage.New(cfg, adapters.Git, adapters.LLM, baseInstructions, projectContext, acceptFragment)
	if err != nil {
		cleanup()
		return nil, err
	}

	remediationRunner := v2remediation.NewRemediationRunner(v2remediation.RemediationRunnerConfig{
		AcceptStage:    acceptStage,
		DecomposeStage: decomposeStage,
		BeadRunner:     &remediationBeadRunner{loop: beadLoop},
		GenerationCap:  v2remediation.DefaultGenerationCap,
		Emitter:        legacyEmitter,
		Presenter:      adapters.Presenter,
	})

	return &Run2LoopComponents{
		PlanStage:             planStage,
		PresentStage:          presentStage,
		PresentSummaryContext: summaryCtx,
		DecomposeStage:        decomposeStage,
		BeadLoop:              beadLoop,
		AcceptStage:           acceptStage,
		RemediationRunner:     remediationRunner,
		Emitter:               typedEmitter,
		StageCommitter:        sc,
		TypedEmitter:          typedEmitter,
	}, nil
}

type remediationBeadRunner struct {
	loop *BeadLoop
}

func (r remediationBeadRunner) Run(ctx context.Context, beads []*bead.Bead) error {
	if r.loop == nil {
		return fmt.Errorf("bead loop required")
	}
	_, err := r.loop.Run(ctx, beads, nil)
	return err
}

type noopValidationRunner struct{}

func (noopValidationRunner) Run(ctx context.Context, command, worktree string) error {
	return nil
}

// CommandValidationRunner executes shell commands and returns errors on failure.
type CommandValidationRunner struct {
	workDir string
}

// NewCommandValidationRunner creates a validation runner that executes shell commands.
func NewCommandValidationRunner(workDir string) *CommandValidationRunner {
	trimmed := strings.TrimSpace(workDir)
	if trimmed == "" {
		trimmed = "."
	}
	return &CommandValidationRunner{workDir: trimmed}
}

// Run executes the command and returns an error if it fails.
func (c *CommandValidationRunner) Run(ctx context.Context, command, worktree string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	dir := c.workDir
	if trimmed := strings.TrimSpace(worktree); trimmed != "" {
		dir = trimmed
	}
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(nil)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("validation command %q failed: %w\nstdout:\n%s\nstderr:\n%s", command, err, stdout.String(), stderr.String())
	}

	return nil
}

// learningsCharCap is the maximum characters of learnings to include in build prompts.
// Matches v1's defaultPromptLearningCharsCap. Only Confirmed and Provisional sections
// are considered; Emerging and Archived are excluded entirely.
const learningsCharCap = 2000

// loadProjectContext loads the project context from CLAUDE.md in the project root,
// and appends a capped subset of learnings from .gromit/LEARNINGS.md.
func loadProjectContext(projectRoot string) (string, error) {
	claudeMDPath := filepath.Join(projectRoot, "CLAUDE.md")
	content, err := os.ReadFile(claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading CLAUDE.md: %w", err)
	}

	result := string(content)

	learningsPath := filepath.Join(projectRoot, ".gromit", "LEARNINGS.md")
	learningsContent, err := os.ReadFile(learningsPath)
	if err == nil && len(learningsContent) > 0 {
		scoped := scopeLearnings(string(learningsContent), learningsCharCap)
		if scoped != "" {
			result += "\n\n## Learnings\n\n" + scoped
		}
	}

	return result, nil
}

// scopeLearnings extracts only the Confirmed and Provisional sections from
// LEARNINGS.md, excluding Emerging and Archived, then caps to maxChars.
func scopeLearnings(raw string, maxChars int) string {
	lines := strings.Split(raw, "\n")
	var kept []string
	include := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			switch {
			case strings.HasPrefix(heading, "confirmed"), strings.HasPrefix(heading, "provisional"):
				include = true
			default:
				// Emerging, Archived, or unknown — skip
				include = false
			}
		}
		if include {
			kept = append(kept, line)
		}
	}

	result := strings.Join(kept, "\n")
	result = strings.TrimSpace(result)

	if len(result) > maxChars {
		result = result[:maxChars]
		// Truncate at last complete line to avoid partial entries.
		if idx := strings.LastIndex(result, "\n"); idx > 0 {
			result = result[:idx]
		}
	}

	return result
}

// loadBaseInstructions loads the base instructions from RULES.md in the project root.
func loadBaseInstructions(projectRoot string) (string, error) {
	rulesMDPath := filepath.Join(projectRoot, "RULES.md")
	content, err := os.ReadFile(rulesMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading RULES.md: %w", err)
	}
	return string(content), nil
}

// loadMethodologyFragments loads methodology-specific build fragments from the project root.
func loadMethodologyFragments(projectRoot string) (buildstage.PromptFragments, error) {
	fragments := buildstage.PromptFragments{}

	// Load standard fragment
	standardContent, err := os.ReadFile(filepath.Join(projectRoot, "build_standard.md"))
	if err != nil && !os.IsNotExist(err) {
		return fragments, fmt.Errorf("reading build_standard.md: %w", err)
	}
	fragments.Standard = string(standardContent)

	// Load TDD fragment
	tddContent, err := os.ReadFile(filepath.Join(projectRoot, "build_tdd.md"))
	if err != nil && !os.IsNotExist(err) {
		return fragments, fmt.Errorf("reading build_tdd.md: %w", err)
	}
	fragments.TDD = string(tddContent)

	// Load refactor fragment
	refactorContent, err := os.ReadFile(filepath.Join(projectRoot, "build_refactor.md"))
	if err != nil && !os.IsNotExist(err) {
		return fragments, fmt.Errorf("reading build_refactor.md: %w", err)
	}
	fragments.Refactor = string(refactorContent)

	return fragments, nil
}

// loadFragment loads a single prompt fragment file from the project root.
// Returns empty string if the file does not exist.
func loadFragment(projectRoot, filename string) (string, error) {
	content, err := os.ReadFile(filepath.Join(projectRoot, filename))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading %s: %w", filename, err)
	}
	return string(content), nil
}
