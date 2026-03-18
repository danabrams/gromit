package stages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"gopkg.in/yaml.v3"
)

// WriteContractsStageConfig configures the WriteContractsStage.
type WriteContractsStageConfig struct {
	// SpecPath is the path to the raw spec markdown file.
	SpecPath string
	// EvidenceDir is the directory where scenario-contracts.yaml will be written.
	EvidenceDir string
	// Store provides access to run storage operations.
	Store *runstore.Store
}

// WriteContractsStage translates spec scenarios into declarative contract assertions
// before implementation begins. It is idempotent when contracts are already written.
// Uses Sonnet (P1) model tier.
type WriteContractsStage struct {
	writer   contract.ContractWriter
	cfg      WriteContractsStageConfig
	budget   *specloop.Budget
	eventLog *runstore.EventLog
}

// NewWriteContractsStage creates a new WriteContractsStage.
func NewWriteContractsStage(writer contract.ContractWriter, cfg WriteContractsStageConfig, budget *specloop.Budget, eventLog *runstore.EventLog) *WriteContractsStage {
	return &WriteContractsStage{
		writer:   writer,
		cfg:      cfg,
		budget:   budget,
		eventLog: eventLog,
	}
}

// Name returns the stage name.
func (s *WriteContractsStage) Name() string { return "write_contracts" }

// Run executes the write-contracts stage.
func (s *WriteContractsStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	// Early guard: EvidenceDir is required to write the contract file.
	if s.cfg.EvidenceDir == "" {
		return specloop.NextAction{}, fmt.Errorf("write_contracts: EvidenceDir is required but empty")
	}

	contractPath := filepath.Join(s.cfg.EvidenceDir, "scenario-contracts.yaml")

	// existingValidationErrors holds validation errors from an existing invalid contract,
	// to be injected into specPacket for retry context. nil means no prior read was done
	// (file didn't exist or was not yet checked).
	var existingValidationErrors []string

	// Before idempotency check: revalidate existing contract if it exists.
	// If the contract is invalid (e.g., references runtime artifacts),
	// reset the flag and proceed with regeneration.
	if rs.ContractsWritten {
		if contractBytes, err := os.ReadFile(contractPath); err == nil {
			// Contract file exists; parse and validate it.
			var existing contract.ScenarioContract
			if err := yaml.Unmarshal(contractBytes, &existing); err == nil {
				errs := contract.ValidateContract(existing)
				if len(errs) == 0 {
					// Contract is valid; maintain idempotency and skip.
					return specloop.NextAction{Kind: specloop.Continue}, nil
				}
				// Contract is invalid; store errors for injection into specPacket below.
				existingValidationErrors = errs
			}
			// Contract file is invalid or unreadable/unparseable; fall through to regenerate.
		} else {
			// No contract file found but flag is set; just skip (file may have been cleaned up).
			return specloop.NextAction{Kind: specloop.Continue}, nil
		}
		// Reset flag to proceed with regeneration.
		rs.ContractsWritten = false
	}

	// Read raw spec markdown to parse scenarios.
	specBytes, err := os.ReadFile(s.cfg.SpecPath)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("read spec file: %w", err)
	}

	scenarios, skipped, err := contract.ParseScenarios(string(specBytes))
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("parse scenarios: %w", err)
	}
	for _, reason := range skipped {
		if s.eventLog != nil {
			s.eventLog.Append(runstore.ContractScenarioSkippedEvent{
				BaseEvent: runstore.BaseEvent{Type: "contract_scenario_skipped", Timestamp: time.Now()},
				Reason:    reason,
			})
		}
	}

	// No scenarios — no-op.
	if len(scenarios) == 0 {
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}

	// Budget check before LLM invocation (checked before reading spec packet to
	// avoid unnecessary I/O when budget is already exhausted).
	if s.budget != nil && s.budget.Exceeded() {
		reason := "budget exhausted: " + s.budget.Reason()
		if s.eventLog != nil {
			s.eventLog.Append(runstore.ContractsBlockedEvent{
				BaseEvent: runstore.BaseEvent{Type: "contracts_blocked", Timestamp: time.Now()},
				Reason:    reason,
			})
		}
		return specloop.NextAction{
			Kind: specloop.Blocked,
			Context: &specloop.FailureContext{
				Failures: []string{reason},
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	// Read compiled spec packet for additional context.
	runDir := s.cfg.Store.RunDir(rs.RunID)
	specPacketBytes, err := os.ReadFile(filepath.Join(runDir, "spec-packet.md"))
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("read spec packet: %w", err)
	}

	specPacket := string(specPacketBytes)

	// Inject validation errors from existing invalid contract into specPacket for retry context.
	// existingValidationErrors is populated during the revalidation phase above.
	if len(existingValidationErrors) > 0 {
		specPacket = "# Prior Contract Validation Errors\n" + strings.Join(existingValidationErrors, "\n") + "\n\n" + specPacket
	}

	// Retry loop: up to 3 total attempts (1 initial + 2 retries).
	const maxAttempts = 3
	const validKeys = "Valid assertion keys: file_exists, file_contains, file_not_modified, file_not_exists, file_not_contains"
	var (
		result           *contract.ScenarioContract
		validationErrors []string
		lastErr          error
	)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		validationErrors = nil
		result, lastErr = s.writer.WriteContracts(ctx, scenarios, specPacket)
		if lastErr != nil {
			// Infrastructure/LLM error; prepend error context to specPacket for next attempt.
			specPacket = "# Prior LLM Error\n" + lastErr.Error() + "\n\n" + string(specPacketBytes)
			continue
		}
		if result == nil {
			// nil contract with nil error means deliberate no-op (e.g. noop writer).
			return specloop.NextAction{Kind: specloop.Continue}, nil
		}

		validationErrors = contract.ValidateContract(*result)
		if len(validationErrors) == 0 {
			// Valid output — break out of retry loop.
			lastErr = nil
			break
		}

		// Prepend validation errors and valid assertion keys to specPacket for next attempt.
		specPacket = "# Prior Validation Errors\n" + strings.Join(validationErrors, "\n") + "\n\n# Valid Assertion Keys\n" + validKeys + "\n\n" + string(specPacketBytes)
	}

	// Determine terminal failure.
	if lastErr != nil || len(validationErrors) > 0 {
		var reason string
		if lastErr != nil {
			reason = fmt.Sprintf("contract writer error: %v", lastErr)
		} else {
			reason = fmt.Sprintf("contract validation failed: %s", strings.Join(validationErrors, "; "))
		}

		if s.eventLog != nil {
			s.eventLog.Append(runstore.ContractsBlockedEvent{
				BaseEvent: runstore.BaseEvent{Type: "contracts_blocked", Timestamp: time.Now()},
				Reason:    reason,
			})
		}
		return specloop.NextAction{
			Kind: specloop.Blocked,
			Context: &specloop.FailureContext{
				Failures: []string{reason},
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	// Write scenario-contracts.yaml to EvidenceDir.
	contractBytes, err := yaml.Marshal(*result)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("marshal contracts: %w", err)
	}
	if err := os.MkdirAll(s.cfg.EvidenceDir, 0o755); err != nil {
		return specloop.NextAction{}, fmt.Errorf("create evidence dir: %w", err)
	}
	if err := os.WriteFile(contractPath, contractBytes, 0o644); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write contracts file: %w", err)
	}

	// Set flag and emit success event.
	rs.ContractsWritten = true

	if s.eventLog != nil {
		s.eventLog.Append(runstore.ContractsWrittenEvent{
			BaseEvent:     runstore.BaseEvent{Type: "contracts_written", Timestamp: time.Now()},
			ScenarioCount: len(result.Scenarios),
		})
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}
