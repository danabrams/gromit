package experiment

import "time"

// Experiment represents an A/B testing experiment for a specific phase.
type Experiment struct {
	ID              string       `yaml:"id"`
	Phase           string       `yaml:"phase"`
	Description     string       `yaml:"description"`
	Created         time.Time    `yaml:"created"`
	Control         *Variant     `yaml:"control"`
	Variants        []*Variant   `yaml:"variants"`
	SuccessCriteria string       `yaml:"success_criteria"`
	ForceVariant    string       `yaml:"force_variant"`
}

// Variant represents a specific variant within an experiment.
type Variant struct {
	ID           string         `yaml:"id"`
	Template     string         `yaml:"template"`
	Budget       *Budget        `yaml:"budget"`
	Model        string         `yaml:"model"`
	ToolCallCap  int            `yaml:"tool_call_cap"`
	Gate         *Gate          `yaml:"gate"`
	Flags        map[string]int `yaml:"flags"`
}

// Budget represents budget constraints for a variant.
type Budget struct {
	MaxChars            int `yaml:"max_chars"`
	LearningCapChars    int `yaml:"learning_cap_chars"`
}

// Gate represents gating criteria for a variant.
type Gate struct {
	MinFilesChanged int `yaml:"min_files_changed"`
}

// Manager manages experiments and provides variant selection.
type Manager struct {
	experimentsByPhase map[string]*Experiment
	stateDir           string
}
