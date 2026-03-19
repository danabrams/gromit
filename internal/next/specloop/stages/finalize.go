package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/next/evidence"
	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// FinalizeStageConfig holds configuration for the FinalizeStage.
type FinalizeStageConfig struct {
	SpecContent string // The spec content for review packet generation
	EvidenceDir string // Path to evidence directory for reading artifact files
}

// FinalizeStage determines the terminal status and handles worktree cleanup.
type FinalizeStage struct {
	gitOps   GitOps
	store    *runstore.Store
	eventLog *runstore.EventLog
	config   *FinalizeStageConfig
}

// NewFinalizeStage creates a new FinalizeStage.
func NewFinalizeStage(gitOps GitOps, store *runstore.Store, eventLog *runstore.EventLog) *FinalizeStage {
	return &FinalizeStage{gitOps: gitOps, store: store, eventLog: eventLog}
}

// NewFinalizeStageWithConfig creates a new FinalizeStage with config.
func NewFinalizeStageWithConfig(gitOps GitOps, store *runstore.Store, eventLog *runstore.EventLog, config *FinalizeStageConfig) *FinalizeStage {
	return &FinalizeStage{gitOps: gitOps, store: store, eventLog: eventLog, config: config}
}

// Name returns the stage name.
func (s *FinalizeStage) Name() string { return "finalize" }

// Run determines the terminal status and saves the final run state.
func (s *FinalizeStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	// If not already in a terminal state (e.g., blocked), determine terminal status
	// by the three quality gates. Individual task failures from earlier cycles do not
	// block ready_for_review if validation, review, and acceptance all passed in the
	// final cycle.
	if rs.Status != runstore.StatusBlocked {
		if rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
			rs.Status = runstore.StatusReadyForReview
		} else {
			rs.Status = runstore.StatusNeedsHuman
		}
	}

	// Emit terminal_state event
	if s.eventLog != nil {
		s.eventLog.Append(runstore.TerminalStateEvent{
			BaseEvent: runstore.BaseEvent{Type: "terminal_state", Timestamp: time.Now()},
			Status:    rs.Status,
			Reason:    rs.TerminalReason,
		})
	}

	rs.EndedAt = time.Now()

	// Generate and write review packet if config is available
	if s.config != nil && s.config.EvidenceDir != "" {
		if err := s.generateReviewPacket(rs); err != nil {
			// Log error but don't fail - continue without packet if generation fails
			if s.eventLog != nil {
				s.eventLog.Append(runstore.BaseEvent{Type: "review_packet_generation_error", Timestamp: time.Now()})
			}
		}
	}

	if err := s.store.Save(rs); err != nil {
		return specloop.NextAction{}, fmt.Errorf("save run state: %w", err)
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}

// generateReviewPacket reads evidence artifacts and generates review packet.
func (s *FinalizeStage) generateReviewPacket(rs *runstore.RunState) error {
	// Read validation.json
	rawValidation, err := readJSONFile(filepath.Join(s.config.EvidenceDir, "validation.json"))
	if err != nil {
		return fmt.Errorf("read validation.json: %w", err)
	}

	// Read review.json
	rawReview, err := readJSONFile(filepath.Join(s.config.EvidenceDir, "review.json"))
	if err != nil {
		return fmt.Errorf("read review.json: %w", err)
	}

	// Read acceptance.json
	rawAcceptance, err := readJSONFile(filepath.Join(s.config.EvidenceDir, "acceptance.json"))
	if err != nil {
		return fmt.Errorf("read acceptance.json: %w", err)
	}

	// Extract validation data into concrete type
	validationResult := reviewpacket.ValidationData{}
	if vm, ok := rawValidation.(map[string]interface{}); ok {
		validationResult.Passed, _ = vm["passed"].(bool)
		validationResult.Checks = jsonIntValue(vm["checks"])
	}

	// Extract acceptance data into concrete type
	acceptanceResult := reviewpacket.AcceptanceData{}
	if am, ok := rawAcceptance.(map[string]interface{}); ok {
		acceptanceResult.Passed = jsonIntValue(am["passed"])
		acceptanceResult.Failed = jsonIntValue(am["failed"])
		acceptanceResult.Unclear = jsonIntValue(am["unclear"])
	}

	// Build review findings map from review data
	reviewFindings := make(map[string][]reviewpacket.ReviewFinding)
	if reviewDataMap, ok := rawReview.(map[string]interface{}); ok {
		for key, value := range reviewDataMap {
			if key == "diff_unavailable" {
				continue
			}
			if items, ok := value.([]interface{}); ok {
				findings := make([]reviewpacket.ReviewFinding, 0, len(items))
				for _, item := range items {
					f := reviewpacket.ReviewFinding{}
					if m, ok := item.(map[string]interface{}); ok {
						f.Message, _ = m["message"].(string)
					}
					findings = append(findings, f)
				}
				reviewFindings[key] = findings
			}
		}
	}

	// Detect degraded flags from review data
	degradedFlags := []string{}
	if reviewDataMap, ok := rawReview.(map[string]interface{}); ok {
		if diffUnavailable, ok := reviewDataMap["diff_unavailable"]; ok {
			if boolVal, ok := diffUnavailable.(bool); ok && boolVal {
				degradedFlags = append(degradedFlags, "diff_unavailable")
			}
		}
	}

	// Detect repeated failure escalation from task lineage
	repeatedFailure := detectRepeatedFailure(rs)

	// Build inputs for generator
	inputs := reviewpacket.Inputs{
		RunID:            rs.RunID,
		SpecTitle:        rs.SpecID, // Use SpecID as title
		SpecContent:      s.config.SpecContent,
		TerminalState:    rs.Status,
		ValidationResult: validationResult,
		ReviewFindings:   reviewFindings,
		AcceptanceResult: acceptanceResult,
		DegradedFlags:    degradedFlags,
		RepairCycles:     rs.Cycle,
		RepeatedFailure:  repeatedFailure,
	}

	// Generate review packet
	gen := &reviewpacket.Generator{}
	outputs, err := gen.Generate(inputs)
	if err != nil {
		return fmt.Errorf("generate review packet: %w", err)
	}

	// Write artifacts using bundler
	bundler := evidence.NewBundler(s.config.EvidenceDir)
	if err := bundler.WriteProductReview(outputs.ProductReview); err != nil {
		return fmt.Errorf("write product review: %w", err)
	}
	if err := bundler.WriteProcessReview(outputs.ProcessReview); err != nil {
		return fmt.Errorf("write process review: %w", err)
	}
	if err := bundler.WriteManualChecklist(outputs.ManualChecklist); err != nil {
		return fmt.Errorf("write manual checklist: %w", err)
	}

	// Render and write markdown artifacts
	productReviewMD := reviewpacket.RenderProductReview(outputs.ProductReview)
	if err := bundler.WriteProductReviewMD(productReviewMD); err != nil {
		return fmt.Errorf("write product review markdown: %w", err)
	}

	processReviewMD := reviewpacket.RenderProcessReview(outputs.ProcessReview)
	if err := bundler.WriteProcessReviewMD(processReviewMD); err != nil {
		return fmt.Errorf("write process review markdown: %w", err)
	}

	return nil
}

// detectRepeatedFailure checks if any task in the lineage has multiple consecutive failures.
func detectRepeatedFailure(rs *runstore.RunState) bool {
	if rs.TaskLineage == nil {
		return false
	}
	for _, entry := range rs.TaskLineage {
		if entry.ConsecutiveFails > 1 {
			return true
		}
	}
	return false
}

// jsonIntValue extracts an integer from a JSON-decoded interface{}, handling both int and float64.
func jsonIntValue(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	default:
		return 0
	}
}

// readJSONFile reads a JSON file and unmarshals it to a map or slice.
func readJSONFile(path string) (interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
