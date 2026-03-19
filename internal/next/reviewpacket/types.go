package reviewpacket

// ValidationData holds the validation result fields needed by the review packet generator.
// This is a local mirror of the relevant fields from validator.FinalResult,
// avoiding an import cycle with the validator package.
type ValidationData struct {
	Passed bool `json:"passed"`
	Checks int  `json:"checks"`
}

// AcceptanceData holds the acceptance result counts needed by the review packet generator.
// This is a local mirror of the relevant aggregate counts from acceptor.AcceptanceResult,
// avoiding an import cycle with the acceptor package.
type AcceptanceData struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Unclear int `json:"unclear"`
}

// ReviewFinding is a minimal representation of a review finding used for counting.
// The review packet generator only needs to know how many findings exist per category,
// so this avoids importing the full review.Finding type.
type ReviewFinding struct {
	Message string `json:"message,omitempty"`
}

// ProductReview is the behavior-focused product review artifact.
type ProductReview struct {
	RunID                 string         `json:"run_id"`
	SpecTitle             string         `json:"spec_title"`
	TerminalState         string         `json:"terminal_state"`
	Summary               string         `json:"summary"`
	BehaviorCards         []BehaviorCard `json:"behavior_cards"`
	Surprises             []string       `json:"surprises,omitempty"`
	IsDiagnostic          bool           `json:"is_diagnostic"`
	BlockerSummary        string         `json:"blocker_summary,omitempty"`
	RecommendedNextAction string         `json:"recommended_next_action,omitempty"`
}

// NormalizeNilFields maps nil slices/maps to empty values.
// See CLAUDE.md nil-field normalization visibility convention.
func (pr *ProductReview) NormalizeNilFields() {
	if pr.BehaviorCards == nil {
		pr.BehaviorCards = []BehaviorCard{}
	}
	if pr.Surprises == nil {
		pr.Surprises = []string{}
	}
}

// BehaviorCard represents a single behavior derived from scenarios or acceptance criteria.
type BehaviorCard struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Given           string   `json:"given,omitempty"`
	When            string   `json:"when,omitempty"`
	Then            string   `json:"then,omitempty"`
	AutomaticStatus string   `json:"automatic_status"`
	EvidenceFiles   []string `json:"evidence_files,omitempty"`
	ManualCheckIDs  []string `json:"manual_check_ids,omitempty"`
	Notes           string   `json:"notes,omitempty"`
}

// NormalizeNilFields converts nil slices to empty slices.
func (bc *BehaviorCard) NormalizeNilFields() {
	if bc.EvidenceFiles == nil {
		bc.EvidenceFiles = []string{}
	}
	if bc.ManualCheckIDs == nil {
		bc.ManualCheckIDs = []string{}
	}
}

// ProcessReview is the deterministic trust and evidence summary.
type ProcessReview struct {
	TrustLevel          string   `json:"trust_level"`
	AutomaticProof      string   `json:"automatic_proof"`
	MachineReview       string   `json:"machine_review"`
	Acceptance          string   `json:"acceptance"`
	DegradedFlags       []string `json:"degraded_flags,omitempty"`
	RepairCycles        int      `json:"repair_cycles"`
	RepeatedFailureFlag bool     `json:"repeated_failure_flag"`
	RecommendedPosture  string   `json:"recommended_posture"`
}

// NormalizeNilFields converts nil slices to empty slices.
func (pr *ProcessReview) NormalizeNilFields() {
	if pr.DegradedFlags == nil {
		pr.DegradedFlags = []string{}
	}
}

// ManualChecklist is the template for manual verification steps.
type ManualChecklist struct {
	Items []ManualCheckItem `json:"items"`
}

// NormalizeNilFields converts nil slices to empty slices.
func (mc *ManualChecklist) NormalizeNilFields() {
	if mc.Items == nil {
		mc.Items = []ManualCheckItem{}
	}
}

// ManualCheckItem represents a single manual verification step.
type ManualCheckItem struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Instructions    string   `json:"instructions"`
	ExpectedResult  string   `json:"expected_result"`
	BehaviorCardIDs []string `json:"behavior_card_ids,omitempty"`
}

// NormalizeNilFields converts nil slices to empty slices.
func (mci *ManualCheckItem) NormalizeNilFields() {
	if mci.BehaviorCardIDs == nil {
		mci.BehaviorCardIDs = []string{}
	}
}

// Inputs holds everything needed to build a review packet.
type Inputs struct {
	RunID            string
	SpecTitle        string
	SpecContent      string
	TerminalState    string
	ValidationResult ValidationData
	ReviewFindings   map[string][]ReviewFinding
	AcceptanceResult AcceptanceData
	DegradedFlags    []string
	RepairCycles     int
	RepeatedFailure  bool
}

// Outputs holds the generated artifacts ready for writing.
type Outputs struct {
	ProductReview   ProductReview
	ProcessReview   ProcessReview
	ManualChecklist ManualChecklist
}
