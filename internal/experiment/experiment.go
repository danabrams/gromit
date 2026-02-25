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

// NewManager creates a new Manager from a slice of experiments.
func NewManager(experiments []*Experiment, stateDir string) *Manager {
	mgr := &Manager{
		experimentsByPhase: make(map[string]*Experiment),
		stateDir:           stateDir,
	}
	for _, exp := range experiments {
		mgr.experimentsByPhase[exp.Phase] = exp
	}
	return mgr
}

// ExperimentForPhase returns the experiment for a given phase, or nil if none exists.
func (m *Manager) ExperimentForPhase(phase string) *Experiment {
	return m.experimentsByPhase[phase]
}
