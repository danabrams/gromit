package planner

// Plan represents a decomposed execution plan for a spec cycle.
type Plan struct {
	SpecID            string    `json:"spec_id"`
	Cycle             int       `json:"cycle"`
	Tasks             []TaskDef `json:"tasks"`
	Kind              string    `json:"kind"` // "original" or "fix"
	ParentCycle       int       `json:"parent_cycle,omitempty"`
	FailuresAddressed []string  `json:"failures_addressed,omitempty"`
}

// TaskDef represents a single task within a plan.
type TaskDef struct {
	TaskID              string   `json:"task_id"`
	Objective           string   `json:"objective"`
	ExpectedTouchedArea []string `json:"expected_touched_area"`
	ProofChecks         []string `json:"proof_checks"`
	ParentCycle         int      `json:"parent_cycle,omitempty"`
	FailuresAddressed   []string `json:"failures_addressed,omitempty"`
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

// NormalizeNilFields maps nil slices to empty values for Plan.
func (p *Plan) NormalizeNilFields() {
	if p.Tasks == nil {
		p.Tasks = []TaskDef{}
	}
	if p.FailuresAddressed == nil {
		p.FailuresAddressed = []string{}
	}
	for i := range p.Tasks {
		p.Tasks[i].NormalizeNilFields()
	}
}

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
