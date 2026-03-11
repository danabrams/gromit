package planner

import (
	"fmt"
	"strconv"
	"strings"
)

// ValidatePlan checks that a plan has non-empty tasks with unique IDs and
// all required fields populated.
func ValidatePlan(p Plan) error {
	if p.Kind != "" && p.Kind != "original" && p.Kind != "fix" {
		return fmt.Errorf("invalid plan kind %q: must be \"original\" or \"fix\"", p.Kind)
	}

	if len(p.Tasks) == 0 {
		return fmt.Errorf("plan must have at least one task")
	}

	seen := make(map[string]bool, len(p.Tasks))
	for i, t := range p.Tasks {
		if t.TaskID == "" {
			return fmt.Errorf("task %d: missing task_id", i)
		}
		if seen[t.TaskID] {
			return fmt.Errorf("duplicate task_id %q", t.TaskID)
		}
		seen[t.TaskID] = true

		if t.Objective == "" {
			return fmt.Errorf("task %s: missing objective", t.TaskID)
		}
		if len(t.ExpectedTouchedArea) == 0 {
			return fmt.Errorf("task %s: missing expected_touched_area", t.TaskID)
		}
		if len(t.ProofChecks) == 0 {
			return fmt.Errorf("task %s: missing proof_checks", t.TaskID)
		}
	}
	return nil
}

// ValidatePlanWithPrior validates the plan and also checks that all task IDs
// are numerically greater than priorMaxID (for cross-cycle continuity).
func ValidatePlanWithPrior(p Plan, priorMaxID string) error {
	if err := ValidatePlan(p); err != nil {
		return err
	}

	priorNum, err := parseTaskNum(priorMaxID)
	if err != nil {
		return fmt.Errorf("invalid prior max ID %q: %w", priorMaxID, err)
	}

	for _, t := range p.Tasks {
		num, err := parseTaskNum(t.TaskID)
		if err != nil {
			return fmt.Errorf("invalid task_id %q: %w", t.TaskID, err)
		}
		if num <= priorNum {
			return fmt.Errorf("task_id %q must be greater than prior max %q", t.TaskID, priorMaxID)
		}
	}
	return nil
}

// parseTaskNum extracts the numeric suffix from a task ID like "t-004".
func parseTaskNum(id string) (int, error) {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected format t-NNN, got %q", id)
	}
	return strconv.Atoi(parts[1])
}
