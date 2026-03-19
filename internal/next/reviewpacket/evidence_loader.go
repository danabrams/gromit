package reviewpacket

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// InputsFromEvidence reconstructs generator Inputs by reading product-review.json,
// process-review.json, manual-checklist.json (or raw evidence: validation.json,
// acceptance.json, review.json) from the evidence directory, spec content from
// specPath, and run metadata from RunState.
func InputsFromEvidence(evidenceDir string, specPath string, run *runstore.RunState) (Inputs, error) {
	// Read spec content from specPath
	specContent, err := os.ReadFile(specPath)
	if err != nil {
		return Inputs{}, fmt.Errorf("read spec from %q: %w", specPath, err)
	}

	// Read validation.json
	validationPath := filepath.Join(evidenceDir, "validation.json")
	validationData, err := os.ReadFile(validationPath)
	if err != nil {
		return Inputs{}, fmt.Errorf("read validation.json: %w", err)
	}

	var validationResult ValidationData
	if err := json.Unmarshal(validationData, &validationResult); err != nil {
		return Inputs{}, fmt.Errorf("unmarshal validation.json: %w", err)
	}

	// Read acceptance.json
	acceptancePath := filepath.Join(evidenceDir, "acceptance.json")
	acceptanceData, err := os.ReadFile(acceptancePath)
	if err != nil {
		return Inputs{}, fmt.Errorf("read acceptance.json: %w", err)
	}

	var acceptanceResult AcceptanceData
	if err := json.Unmarshal(acceptanceData, &acceptanceResult); err != nil {
		return Inputs{}, fmt.Errorf("unmarshal acceptance.json: %w", err)
	}

	// Read review.json to extract review findings
	reviewPath := filepath.Join(evidenceDir, "review.json")
	reviewData, err := os.ReadFile(reviewPath)
	if err != nil {
		return Inputs{}, fmt.Errorf("read review.json: %w", err)
	}

	// Parse review.json to extract review findings by category
	var reviewRaw map[string]interface{}
	if err := json.Unmarshal(reviewData, &reviewRaw); err != nil {
		return Inputs{}, fmt.Errorf("unmarshal review.json: %w", err)
	}

	reviewFindings := make(map[string][]ReviewFinding)

	// Extract review findings from known categories in review.json
	for category := range reviewRaw {
		if category == "diff_unavailable" {
			continue // skip non-findings fields
		}

		if findingsRaw, ok := reviewRaw[category]; ok {
			if findingsArray, ok := findingsRaw.([]interface{}); ok {
				var findings []ReviewFinding
				for _, item := range findingsArray {
					if itemMap, ok := item.(map[string]interface{}); ok {
						var finding ReviewFinding
						if msg, ok := itemMap["message"].(string); ok {
							finding.Message = msg
						}
						findings = append(findings, finding)
					}
				}
				if len(findings) > 0 {
					reviewFindings[category] = findings
				}
			}
		}
	}

	// If no findings were extracted, ensure we have at least empty maps for expected categories
	if len(reviewFindings) == 0 {
		reviewFindings["info"] = []ReviewFinding{}
	}

	// Extract degraded flags from run state
	degradedFlags := run.ReviewFindings
	if degradedFlags == nil {
		degradedFlags = []string{}
	}

	// Determine if this is a repeated failure from run state by checking if any
	// failure in the failure history or task lineage has repeated occurrences
	repeatedFailure := false
	for _, count := range run.FailureHistory {
		if count > 1 {
			repeatedFailure = true
			break
		}
	}
	if !repeatedFailure {
		for _, entry := range run.TaskLineage {
			if entry.ConsecutiveFails > 1 {
				repeatedFailure = true
				break
			}
		}
	}

	// Build Inputs struct
	inputs := Inputs{
		RunID:            run.RunID,
		SpecTitle:        run.SpecID, // Use spec ID as title; caller can override if needed
		SpecContent:      string(specContent),
		TerminalState:    run.Status,
		ValidationResult: validationResult,
		ReviewFindings:   reviewFindings,
		AcceptanceResult: acceptanceResult,
		DegradedFlags:    degradedFlags,
		RepairCycles:     run.TotalReplans,
		RepeatedFailure:  repeatedFailure,
	}

	return inputs, nil
}
