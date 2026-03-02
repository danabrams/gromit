package prepare

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/readiness"
)

// maxScopeFiles is the file-count cap: beads with more than this many expected outputs
// are blocked by the scope gate to prevent oversized single-iteration work.
// Based on the decomposition rule: split beads touching 6+ files.
const maxScopeFiles = 5

// Prechecker determines whether a bead's acceptance criteria are already satisfied.
// When Check returns true, the Gate returns Skip to close the bead without build work.
type Prechecker interface {
	Check(ctx context.Context, b *bead.Bead) (bool, error)
}

// StuckDetector determines whether a bead has exceeded its failure threshold.
// When IsStuck returns true, the Gate returns Block to surface the bead for decomposition.
type StuckDetector interface {
	IsStuck(ctx context.Context, b *bead.Bead) (bool, error)
}

// Decomposer proactively decomposes an oversized bead into sub-beads.
// When Decompose succeeds, the original bead is closed and sub-beads are created;
// the Gate then returns Skip so the orchestrator moves to the next bead.
type Decomposer interface {
	Decompose(ctx context.Context, b *bead.Bead) error
}

// DataQualityBlocker determines whether data quality requirements are met for proceeding.
// When ShouldBlock returns true, the Gate returns Block to prevent further processing.
type DataQualityBlocker interface {
	ShouldBlock(ctx context.Context, b *bead.Bead) (bool, string, error) // blocked, reason, error
}

// Gate implements pipeline.Stage for Stage 1: gate decisions.
// It runs precheck, stuck-bead detection, and scope gate
// before any LLM build invocation.
// If precheck passes (work already done), returns Skip.
// If stuck (failure threshold exceeded), returns Block.
// If scope too large (expected outputs > maxScopeFiles), returns Block.
// Keyword-based proactive decomposition is intentionally disabled to avoid
// decomposition loops from title/description heuristics.
// If data quality blocked, returns Block.
// Otherwise returns Proceed.
type Gate struct {
	events.EmitterMixin                      // provides Emitter field and SetEmitter method
	precheck            Prechecker           // optional; nil means skip precheck
	readiness           readiness.Assessor   // optional; nil means skip readiness assessment
	stuck               StuckDetector        // optional; nil means skip stuck detection
	decomposer          Decomposer           // optional; used only by scope-gate decomposition
	dataQualityChecker  DataQualityBlocker   // optional; nil means skip data quality checks
	specSPCBlocker      *SpecSPCBlocker      // optional; nil means skip spec-level SPC blocking
	criteriaEnricher    *LLMCriteriaEnricher // optional; nil means skip criteria enrichment
	output              io.Writer
}

// Compile-time check: *Gate must implement pipeline.Stage.
var _ pipeline.Stage = (*Gate)(nil)

// New creates a Gate stage with the given output writer.
// output receives diagnostic messages; pass io.Discard to suppress.
func New(output io.Writer) *Gate {
	return &Gate{output: output}
}

// WithEmitter attaches an EventEmitter for log events.
func (g *Gate) WithEmitter(emitter *events.Emitter) *Gate {
	g.EmitterMixin.SetEmitter(emitter)
	return g
}

// WithPrechecker configures an optional Prechecker for detecting already-completed work.
func (g *Gate) WithPrechecker(p Prechecker) *Gate {
	g.precheck = p
	return g
}

// WithReadinessAssessor configures an optional ReadinessAssessor for readiness gating.
func (g *Gate) WithReadinessAssessor(r readiness.Assessor) *Gate {
	g.readiness = r
	return g
}

// HasReadinessAssessor returns true if a ReadinessAssessor is wired in.
func (g *Gate) HasReadinessAssessor() bool {
	return g.readiness != nil
}

// WithStuckDetector configures an optional StuckDetector for detecting exceeded failure thresholds.
func (g *Gate) WithStuckDetector(s StuckDetector) *Gate {
	g.stuck = s
	return g
}

// WithDecomposer configures an optional Decomposer for scope-gate decomposition of oversized beads.
func (g *Gate) WithDecomposer(d Decomposer) *Gate {
	g.decomposer = d
	return g
}

// WithDataQualityBlocker configures an optional DataQualityBlocker for data completeness checks.
func (g *Gate) WithDataQualityBlocker(dq DataQualityBlocker) *Gate {
	g.dataQualityChecker = dq
	return g
}

// WithSpecSPCBlocker configures an optional SpecSPCBlocker for spec-level SPC anomaly blocking.
func (g *Gate) WithSpecSPCBlocker(sb *SpecSPCBlocker) *Gate {
	g.specSPCBlocker = sb
	return g
}

// WithCriteriaEnricher configures an optional LLMCriteriaEnricher for auto-generating acceptance criteria.
func (g *Gate) WithCriteriaEnricher(e *LLMCriteriaEnricher) *Gate {
	g.criteriaEnricher = e
	return g
}

// HasDecomposer returns true if a Decomposer is wired in, false otherwise.
func (g *Gate) HasDecomposer() bool {
	return g.decomposer != nil
}

// HasDataQualityBlocker returns true if a DataQualityBlocker is wired in, false otherwise.
func (g *Gate) HasDataQualityBlocker() bool {
	return g.dataQualityChecker != nil
}

// HasSpecSPCBlocker returns true if a SpecSPCBlocker is wired in, false otherwise.
func (g *Gate) HasSpecSPCBlocker() bool {
	return g.specSPCBlocker != nil
}

// Run executes the gate stage.
// It runs precheck (Skip if work already done), stuck detection (Block if threshold exceeded),
// and scope gate (Block/Skip based on oversized decomposition outcome),
// and returns Proceed otherwise.
func (g *Gate) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if in.Bead == nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	complexityRouting := resolveComplexityRouting(in)
	gateBlockReason := ""
	overrideEnabled := in.Config != nil && in.Config.ReadinessEmergencyOverride

	// Precheck: skip beads whose acceptance criteria are already satisfied.
	// Runs before stuck detection so completed work is always closed promptly.
	if g.precheck != nil && !shouldBypassPrecheck(in.Bead, in.Config) {
		done, err := g.precheck.Check(ctx, in.Bead)
		if err != nil {
			g.Log("warning", "Warning: precheck failed for bead %s: %v", in.Bead.ID, err)
		} else if done {
			if in.Emitter != nil {
				in.Emitter.Emit(&events.GateSkipEvent{
					BeadID: in.Bead.ID,
					Reason: "precheck_passed",
				})
			}
			return pipeline.Output{
				Decision:          pipeline.Skip,
				ComplexityRouting: complexityRouting,
			}, nil
		}
	}

	// Readiness check: block beads that are not ready before stuck detection.
	if g.readiness != nil {
		readinessBead, usedFallback := beadForReadinessAssessment(in.Bead)
		if usedFallback {
			g.Log("info", "Readiness fallback: synthesized expected output from title for from-review bead %s", in.Bead.ID)
		}
		assessment, err := g.readiness.Assess(ctx, readinessBead)
		if err != nil {
			g.Log("warning", "Warning: readiness assessment failed for bead %s: %v", in.Bead.ID, err)
		} else if assessment.Status == readiness.StatusNotReady {
			gateBlockReason = assessment.Reason
			if gateBlockReason == "" {
				gateBlockReason = string(readiness.StatusNotReady)
			}
			if overrideEnabled {
				overrideReason := gateBlockReason
				if normalized, _ := readiness.NormalizeReason(overrideReason); normalized != "" {
					overrideReason = normalized
				}
				g.Log("warning", "Readiness emergency override: bead %s (%s) allowed through despite %s", in.Bead.ID, in.Bead.Title, overrideReason)
				gateBlockReason = ""
			}
		}
	}

	// Stuck detection: block beads that have exceeded the failure threshold.
	if g.stuck != nil && gateBlockReason == "" {
		stuck, err := g.stuck.IsStuck(ctx, in.Bead)
		if err != nil {
			g.Log("warning", "Warning: stuck detection failed for bead %s: %v", in.Bead.ID, err)
		} else if stuck {
			if in.Emitter != nil {
				in.Emitter.Emit(&events.GateStuckEvent{
					BeadID: in.Bead.ID,
					Reason: "failure_threshold_exceeded",
				})
			}
			return pipeline.Output{
				Decision:          pipeline.Block,
				GateBlockReason:   "failure_threshold_exceeded",
				ComplexityRouting: complexityRouting,
			}, nil
		}
	}
	if gateBlockReason != "" {
		if gateBlockReason == ReasonCriteriaAmbiguous && g.decomposer != nil {
			g.Log("info", "Readiness gate: bead %s has ambiguous criteria, attempting decomposition", in.Bead.ID)
			if err := g.decomposer.Decompose(ctx, in.Bead); err == nil {
				g.Log("info", "Readiness gate: decomposition succeeded for bead %s, skipping parent bead", in.Bead.ID)
				return pipeline.Output{
					Decision:          pipeline.Skip,
					ComplexityRouting: complexityRouting,
				}, nil
			} else {
				g.Log("warning", "Warning: readiness decomposition failed for bead %s: %v, falling back to block", in.Bead.ID, err)
			}
		}
		if in.Emitter != nil {
			eventReason, _ := readiness.NormalizeReason(gateBlockReason)
			if eventReason == "" {
				eventReason = gateBlockReason
			}
			in.Emitter.Emit(&events.GateReadinessBlockEvent{
				BeadID: in.Bead.ID,
				Reason: eventReason,
				Time:   time.Now(),
			})
		}
		return pipeline.Output{
			Decision:          pipeline.Block,
			GateBlockReason:   gateBlockReason,
			ComplexityRouting: complexityRouting,
		}, nil
	}

	// Data quality check: block beads if data quality requirements are not met.
	if gateBlockReason == "" && g.dataQualityChecker != nil {
		blocked, reason, err := g.dataQualityChecker.ShouldBlock(ctx, in.Bead)
		if err != nil {
			g.Log("warning", "Warning: data quality check failed for bead %s: %v", in.Bead.ID, err)
		} else if blocked {
			g.Log("warning", "Data quality block for bead %s: %s", in.Bead.ID, reason)
			if in.Emitter != nil {
				in.Emitter.Emit(&events.GateBlockEvent{
					BeadID: in.Bead.ID,
					Reason: reason,
					Time:   time.Now(),
				})
			}
			return pipeline.Output{
				Decision:          pipeline.Block,
				GateBlockReason:   reason,
				ComplexityRouting: complexityRouting,
			}, nil
		}
	}

	// Spec-level SPC blocking: block beads with high-severity rework anomalies in their spec.
	if gateBlockReason == "" && g.specSPCBlocker != nil {
		blocked, reason, err := g.specSPCBlocker.ShouldBlock(ctx, in.Bead)
		if err != nil {
			g.Log("warning", "Warning: spec SPC block check failed for bead %s: %v", in.Bead.ID, err)
		} else if blocked {
			g.Log("warning", "Spec SPC block for bead %s: %s", in.Bead.ID, reason)
			if in.Emitter != nil {
				in.Emitter.Emit(&events.GateBlockEvent{
					BeadID: in.Bead.ID,
					Reason: reason,
					Time:   time.Now(),
				})
			}
			return pipeline.Output{
				Decision:          pipeline.Block,
				GateBlockReason:   reason,
				ComplexityRouting: complexityRouting,
			}, nil
		}
	}

	// Scope gate: try to handle oversized beads via decomposition, fallback to block.
	if gateBlockReason == "" {
		scopeGateDecision, scopeGateErr := g.runScopeGate(ctx, in)
		if scopeGateErr != nil {
			return pipeline.Output{}, scopeGateErr
		}
		if scopeGateDecision != nil {
			scopeGateDecision.ComplexityRouting = complexityRouting
			return *scopeGateDecision, nil
		}
	}

	return pipeline.Output{
		Decision:          pipeline.Proceed,
		ComplexityRouting: complexityRouting,
	}, nil
}

func beadForReadinessAssessment(b *bead.Bead) (*bead.Bead, bool) {
	if b == nil {
		return nil, false
	}
	if !bead.HasLabel(b.Labels, "from-review") {
		return b, false
	}
	if len(effectiveCriteria(b)) > 0 {
		return b, false
	}
	trimmedTitle := strings.TrimSpace(b.Title)
	if trimmedTitle == "" {
		return b, false
	}
	clone := *b
	clone.ExpectedOutputs = []string{trimmedTitle}
	return &clone, true
}

func shouldBypassPrecheck(b *bead.Bead, cfg *config.Config) bool {
	if b == nil {
		return false
	}

	issueTypes := precheckBypassIssueTypes(cfg)
	labelValues := precheckBypassLabelValues(cfg)

	normalizedType := strings.ToLower(strings.TrimSpace(b.Type))
	if _, ok := issueTypes[normalizedType]; ok {
		return true
	}

	for _, label := range b.Labels {
		if shouldBypassPrecheckLabel(label, labelValues) {
			return true
		}
	}

	return false
}

func shouldBypassPrecheckLabel(label string, allowlist map[string]struct{}) bool {
	normalized := strings.ToLower(strings.TrimSpace(label))
	if normalized == "" {
		return false
	}
	if _, ok := allowlist[normalized]; ok {
		return true
	}

	parts := strings.Split(normalized, ":")
	if len(parts) < 2 {
		return false
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	_, ok := allowlist[last]
	return ok
}

func precheckBypassIssueTypes(cfg *config.Config) map[string]struct{} {
	values := config.PrecheckConfig{}.EffectiveBypassIssueTypes()
	if cfg != nil {
		values = cfg.Precheck.EffectiveBypassIssueTypes()
	}
	return normalizePrecheckBypassValues(values)
}

func precheckBypassLabelValues(cfg *config.Config) map[string]struct{} {
	values := config.PrecheckConfig{}.EffectiveBypassLabels()
	if cfg != nil {
		values = cfg.Precheck.EffectiveBypassLabels()
	}
	return normalizePrecheckBypassValues(values)
}

func normalizePrecheckBypassValues(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		out[normalized] = struct{}{}
	}
	return out
}

func resolveComplexityRouting(in pipeline.Input) pipeline.ComplexityRouting {
	if normalized, ok := normalizeComplexity(in.Complexity); ok {
		return pipeline.ComplexityRouting{
			Complexity:               normalized,
			ComplexitySource:         "scope_estimate",
			ComplexityFallbackReason: "none",
		}
	}
	if normalized, ok := complexityFromLabels(in.Bead); ok {
		return pipeline.ComplexityRouting{
			Complexity:               normalized,
			ComplexitySource:         "label",
			ComplexityFallbackReason: "scope_unavailable",
		}
	}
	return pipeline.ComplexityRouting{
		Complexity:               "medium",
		ComplexitySource:         "default",
		ComplexityFallbackReason: "scope_and_label_unavailable",
	}
}

func normalizeComplexity(complexity string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(complexity))
	switch normalized {
	case "low", "medium", "high":
		return normalized, true
	}
	return "", false
}

func complexityFromLabels(b *bead.Bead) (string, bool) {
	if b == nil {
		return "", false
	}
	for _, label := range b.Labels {
		normalizedLabel := strings.ToLower(strings.TrimSpace(label))
		if !strings.HasPrefix(normalizedLabel, "complexity:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(normalizedLabel, "complexity:"))
		if normalized, ok := normalizeComplexity(value); ok {
			return normalized, true
		}
	}
	return "", false
}

// runScopeGate evaluates scope gate rules and attempts decomposition if needed.
// Returns a non-nil decision if scope gate blocks/skips; nil means scope gate allows progression.
// Returns an error if decomposition fails.
func (g *Gate) runScopeGate(ctx context.Context, in pipeline.Input) (*pipeline.Output, error) {
	if in.Config == nil || !in.Config.ScopeCheck.Enabled {
		return nil, nil
	}

	blockOversized := in.Config.ScopeCheck.BlockOversized == nil || *in.Config.ScopeCheck.BlockOversized
	fileCount := bead.EstimatedFileCount(in.Bead)

	if !blockOversized || fileCount <= maxScopeFiles {
		return nil, nil
	}

	action := "block"
	if g.decomposer != nil {
		action = "decompose"
	}
	if in.Emitter != nil {
		in.Emitter.Emit(&events.GateScopeEvent{
			BeadID:    in.Bead.ID,
			FileCount: fileCount,
			MaxFiles:  maxScopeFiles,
			Action:    action,
			Time:      time.Now(),
		})
	}

	// Bead exceeds scope limit. Try decomposition if available.
	// Scope-based decomposition applies to child beads too: the finite
	// expected-outputs count bounds recursion depth naturally.
	if g.decomposer != nil {
		g.Log("info", "Scope gate: bead %s has %d expected outputs (max %d), attempting decomposition",
			in.Bead.ID, fileCount, maxScopeFiles)
		if err := g.decomposer.Decompose(ctx, in.Bead); err != nil {
			g.Log("warning", "Warning: decomposition failed for bead %s: %v, falling back to block", in.Bead.ID, err)
			if in.Emitter != nil {
				in.Emitter.Emit(&events.GateBlockEvent{
					BeadID: in.Bead.ID,
					Reason: "scope",
				})
			}
			return &pipeline.Output{
				Decision:        pipeline.Block,
				GateBlockReason: "scope",
			}, nil
		}
		g.Log("info", "Scope gate: decomposition succeeded for bead %s, skipping parent bead", in.Bead.ID)
		return &pipeline.Output{Decision: pipeline.Skip}, nil
	}

	// Decomposition not available or bead is child: block.
	g.Log("warning", "Scope gate: bead %s has %d expected outputs (max %d), blocking",
		in.Bead.ID, fileCount, maxScopeFiles)
	if in.Emitter != nil {
		in.Emitter.Emit(&events.GateBlockEvent{
			BeadID: in.Bead.ID,
			Reason: "scope",
		})
	}
	return &pipeline.Output{
		Decision:        pipeline.Block,
		GateBlockReason: "scope",
	}, nil
}
