package planner

// Plan represents a decomposed execution plan for a spec cycle.
type Plan struct {
	SpecID                string    `json:"spec_id"`
	Cycle                 int       `json:"cycle"`
	Tasks                 []TaskDef `json:"tasks"`
	Kind                  string    `json:"kind"` // "original" or "fix"
	ParentCycle           int       `json:"parent_cycle,omitempty"`
	FailuresAddressed     []string  `json:"failures_addressed,omitempty"`
	ArchitectureDecisions []string  `json:"architecture_decisions,omitempty"`
}

// TaskDef represents a single task within a plan.
type TaskDef struct {
	TaskID              string   `json:"task_id"`
	Objective           string   `json:"objective"`
	ExpectedTouchedArea []string `json:"expected_touched_area"`
	ProofChecks         []string `json:"proof_checks"`
	ParentCycle         int      `json:"parent_cycle,omitempty"`
	FailuresAddressed   []string `json:"failures_addressed,omitempty"`
	Fixes               string   `json:"fixes,omitempty"`
}

// CompletedTask summarizes a task execution from a prior cycle.
type CompletedTask struct {
	TaskID            string   `json:"task_id"`
	Attempts          int      `json:"attempts"`
	FilesChanged      []string `json:"files_changed"`
	ValidationOutcome string   `json:"validation_outcome"`
}

// FixPlanRequest contains everything needed to generate a fix plan.
type FixPlanRequest struct {
	OriginalPlan            Plan            `json:"original_plan"`
	CompletedTasks          []CompletedTask `json:"completed_tasks"`
	Failures                []string        `json:"failures"`
	CurrentDiff             string          `json:"current_diff"`
	Cycle                   int             `json:"cycle"`
	PriorMaxTaskID          string          `json:"prior_max_task_id,omitempty"`
	SpecConstraints         string          `json:"spec_constraints,omitempty"`
	ArchitectureConstraints []string        `json:"architecture_constraints,omitempty"`
	SpecPacket              string          `json:"spec_packet,omitempty"`
	PlaybookHeuristics      string          `json:"playbook_heuristics,omitempty"`
	DoctrineRules           string          `json:"doctrine_rules,omitempty"`
	ReviewerGuidance        string          `json:"reviewer_guidance,omitempty"` // Human reviewer instructions from review-outcome.json; empty if none
}

// TaskByID returns the task with the given ID and true, or a zero TaskDef and false.
func (p Plan) TaskByID(id string) (TaskDef, bool) {
	for _, t := range p.Tasks {
		if t.TaskID == id {
			return t, true
		}
	}
	return TaskDef{}, false
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for Plan.
func (p *Plan) NormalizeNilFields() {
	if p.Tasks == nil {
		p.Tasks = []TaskDef{}
	}
	if p.FailuresAddressed == nil {
		p.FailuresAddressed = []string{}
	}
	if p.ArchitectureDecisions == nil {
		p.ArchitectureDecisions = []string{}
	}
	for i := range p.Tasks {
		p.Tasks[i].NormalizeNilFields()
	}
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for TaskDef.
func (td *TaskDef) NormalizeNilFields() {
	if td.ExpectedTouchedArea == nil {
		td.ExpectedTouchedArea = []string{}
	}
	if td.ProofChecks == nil {
		td.ProofChecks = []string{}
	}
	if td.FailuresAddressed == nil {
		td.FailuresAddressed = []string{}
	}
}
