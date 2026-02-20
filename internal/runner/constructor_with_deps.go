package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/policy"
	"github.com/danabrams/gromit/internal/runner/reviewpkg"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// Deps holds injectable dependencies for a Runner, used for testing.
type Deps struct {
	Beads             BeadClient
	Router            *provider.Router
	Analyzer          FailureAnalyzer
	Renderer          PromptRenderer
	Logger            IterationLogger
	EscalationPolicy  policy.EscalationPolicy
	MethodologyPolicy policy.MethodologyPolicy
	ValidationPolicy  policy.ValidationPolicy
	StuckPolicy       policy.StuckPolicy
	CmdRunner         func(ctx context.Context, command string, workDir string) (stdout string, stderr string, exitCode int, err error)
	ArgvRunner        ArgvRunnerFn
}

// newRunnerWithDepsImpl creates a runner with explicitly provided dependencies.
// This is primarily intended for testing, where you want to inject mocks.
func newRunnerWithDepsImpl(cfg *config.Config, output io.Writer, gromitDir string, deps Deps) (*Runner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	// Apply config defaults to ensure consistent behavior even when config is
	// partially initialized in tests. This prevents accidental precheck execution
	// or other features in tests that don't explicitly test them.
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if output == nil {
		output = os.Stdout
	}
	// Wrap output in synchronized writer for thread-safe writes
	syncOut := newSyncWriter(output)

	// Create logger if not provided and Logs path is configured
	iterLogger := deps.Logger
	if iterLogger == nil && cfg.Paths.Logs != "" {
		log, err := logger.NewLogger(cfg.Paths.Logs)
		if err != nil {
			// Log warning but continue - logging is optional
			_, _ = fmt.Fprintf(output, "Warning: could not create logger: %v\n", err)
		} else {
			iterLogger = log
		}
	}

	// Use provided Router
	router := deps.Router

	// Create invoker with router adapter (nil-safe: if router is nil, invoker handles it)
	stallTimeoutFn := makeStallTimeoutFn(cfg)
	inv := buildInvoker(router, syncOut, stallTimeoutFn, cfg)

	cmdRunner := defaultCmdRunner
	if deps.CmdRunner != nil {
		cmdRunner = deps.CmdRunner
	}
	argvRunner := defaultArgvRunner
	if deps.ArgvRunner != nil {
		argvRunner = deps.ArgvRunner
	}

	stuckPolicy := deps.StuckPolicy
	if stuckPolicy == nil {
		stuckPolicy = policy.NewConfigStuckPolicy(cfg)
	}

	r := &Runner{
		cfg:               cfg,
		beads:             deps.Beads,
		router:            router,
		invoker:           inv,
		analyzer:          deps.Analyzer,
		renderer:          deps.Renderer,
		logger:            iterLogger,
		escalationPolicy:  deps.EscalationPolicy,
		methodologyPolicy: deps.MethodologyPolicy,
		validationPolicy:  deps.ValidationPolicy,
		stuckPolicy:       stuckPolicy,
		output:            syncOut,
		syncOut:           syncOut,
		gromitDir:         gromitDir,
		gitDiffFn:         getGitDiff,
		gitHeadFn:         getGitHead,
		cmdRunnerFn:       cmdRunner,
		argvRunnerFn:      argvRunner,
		processChecker:    IsProcessAlive,
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return net.DefaultResolver.LookupHost(ctx, host)
		},
		lookPathFn:    exec.LookPath,
		stdinStatFn:   os.Stdin.Stat,
		promptYesNoFn: func(question string) (bool, error) { return promptYesNo(os.Stdin, syncOut, question) },
	}
	r.escalationHandler = escalation.NewHandler(cfg, deps.Analyzer, deps.Beads, r.DecomposeTask, r.CreateSubBeads, r.log, r.showPartialProgress)
	r.validationRunner = validation.NewRunner(cfg, cmdRunner, r.autoFixFn, r.makeValidationExecuteFn())
	r.methodologyExec = r.makeMethodologyExec()
	r.tddOrchestrator = r.makeTDDOrchestrator()
	r.cycleOrchestrator = &cycleOrchestrator{runner: r}
	if cfg.Methodology.Granularity == config.MethodologyGranularitySpec {
		r.specOrchestrator = newSpecOrchestrator(r)
	}
	if cfg.SpecGate.IsEnabled() {
		gate, err := r.buildSpecGate()
		if err != nil {
			return nil, err
		}
		r.specGate = gate
	}
	r.reviewer = reviewpkg.NewReviewer(cfg, router, deps.Beads, deps.Renderer, r.gitDiffFn, iterLogger)
	r.reviewer.SetLogFn(r.log)
	r.reviewer.SetValidateFn(r.makeReviewValidateFn())
	return r, nil
}

// IterationResult captures the outcome of one loop iteration.
// Type alias for backward compatibility — canonical definition is in runtypes.
type IterationResult = runtypes.IterationResult

// SubTask represents a single sub-task from task decomposition.
// Type alias for backward compatibility — canonical definition is in runtypes.
type SubTask = runtypes.SubTask

// ArgvRunnerFn runs an executable with argv and returns output plus exit metadata.
// Type alias for backward compatibility — canonical definition is in runtypes.
type ArgvRunnerFn = runtypes.ArgvRunnerFn
