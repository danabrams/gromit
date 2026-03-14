package e2e

// Contract defines a scenario test contract loaded from a YAML file.
type Contract struct {
	Name      string `yaml:"name"`
	Scenario  int    `yaml:"scenario"`
	Spec      string `yaml:"spec"`
	Fixture   string `yaml:"fixture"`
	StoreDir  string `yaml:"store_dir"`
	Policy    string `yaml:"policy"`
	ExtraFlags []string `yaml:"extra_flags"`
	InlinePolicy string `yaml:"inline_policy"`
	DependsOnScenario int `yaml:"depends_on_scenario"`
	Concurrent bool `yaml:"concurrent"`

	FixtureReset FixtureReset `yaml:"fixture_reset"`
	Assertions   []Assertion  `yaml:"assertions"`
}

// FixtureReset describes how to reset the fixture directory before running a scenario.
type FixtureReset struct {
	GitFiles    []GitFileRestore `yaml:"git_files"`
	RemoveFiles []string         `yaml:"remove_files"`
	AddFiles    []FileCopy       `yaml:"add_files"`
}

// GitFileRestore restores specific files to a given git commit state.
type GitFileRestore struct {
	Commit string   `yaml:"commit"`
	Files  []string `yaml:"files"`
}

// FileCopy copies a file from Src to Dst during fixture reset.
type FileCopy struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
}

// Assertion is a single-key map — only one key set per assertion.
type Assertion struct {
	// Run state
	Status                string   `yaml:"status"`
	StatusOneOf           []string `yaml:"status_one_of"`
	TerminalReason        string   `yaml:"terminal_reason"`
	FinalValidationPassed *bool    `yaml:"final_validation_passed"`
	CostUSDGt             *float64 `yaml:"cost_usd_gt"`
	ReplansGte            *int     `yaml:"replans_gte"`
	ReplansEq             *int     `yaml:"replans_eq"`
	CycleEq               *int     `yaml:"cycle_eq"`
	EndedAtSet            *bool    `yaml:"ended_at_set"`

	// Evidence
	AcceptanceAllPass       *bool `yaml:"acceptance_all_pass"`
	ValidationPass          *bool `yaml:"validation_pass"`
	NoErrorSeverityFindings *bool `yaml:"no_error_severity_findings"`
	InvocationsCountGte     *int  `yaml:"invocations_count_gte"`

	// Tasks
	AllTasksAttempted           *bool  `yaml:"all_tasks_attempted"`
	FilesChangedNonempty        *bool  `yaml:"files_changed_nonempty"`
	FilesChangedNeverContains   string `yaml:"files_changed_never_contains"`
	AnyTaskFilesChangedContains string `yaml:"any_task_files_changed_contains"`

	// Filesystem
	FileContains    *FileContainsAssertion `yaml:"file_contains"`
	FileNotModified string                 `yaml:"file_not_modified"`

	// Events
	EventsContainType string `yaml:"events_contain_type"`

	// CLI
	ExecShowContains        string `yaml:"exec_show_contains"`
	ExecShowNotContains     string `yaml:"exec_show_not_contains"`
	ExecShowFullContains    string `yaml:"exec_show_full_contains"`
	ExecShowFullNotContains string `yaml:"exec_show_full_not_contains"`
	ExecListContains        string `yaml:"exec_list_contains"`
	SpecListContains        string `yaml:"spec_list_contains"`
}

// FileContainsAssertion checks that a file at Path matches Pattern.
type FileContainsAssertion struct {
	Path    string `yaml:"path"`
	Pattern string `yaml:"pattern"`
}
