package pipeline

import "testing"

func TestFormatCommitMessage(t *testing.T) {
	tests := []struct {
		name      string
		beadID    string
		stageName string
		iteration int
		decision  string
		want      string
	}{
		{
			name:      "bead-level stage",
			beadID:    "003",
			stageName: "build",
			iteration: 1,
			decision:  "Proceed",
			want:      "[bead:003/build/iter:1] Proceed",
		},
		{
			name:      "spec-level stage",
			beadID:    "",
			stageName: "plan",
			iteration: 1,
			decision:  "Proceed",
			want:      "[spec/plan/iter:1] Proceed",
		},
		{
			name:      "validation failure",
			beadID:    "003",
			stageName: "validate",
			iteration: 2,
			decision:  "Fail",
			want:      "[bead:003/validate/iter:2] Fail",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCommitMessage(tt.beadID, tt.stageName, tt.iteration, tt.decision)
			if got != tt.want {
				t.Errorf("FormatCommitMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseCommitMessage(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantID    string
		wantStage string
		wantIter  int
		wantDec   string
		wantOK    bool
	}{
		{
			name:      "bead-level",
			input:     "[bead:003/build/iter:1] Proceed",
			wantID:    "003",
			wantStage: "build",
			wantIter:  1,
			wantDec:   "Proceed",
			wantOK:    true,
		},
		{
			name:      "spec-level",
			input:     "[spec/plan/iter:1] Proceed",
			wantID:    "",
			wantStage: "plan",
			wantIter:  1,
			wantDec:   "Proceed",
			wantOK:    true,
		},
		{
			name:   "not a structured message",
			input:  "[gromit] bead 003: some title",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := ParseCommitMessage(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ParseCommitMessage() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if parsed.BeadID != tt.wantID {
				t.Errorf("BeadID = %q, want %q", parsed.BeadID, tt.wantID)
			}
			if parsed.StageName != tt.wantStage {
				t.Errorf("StageName = %q, want %q", parsed.StageName, tt.wantStage)
			}
			if parsed.Iteration != tt.wantIter {
				t.Errorf("Iteration = %d, want %d", parsed.Iteration, tt.wantIter)
			}
			if parsed.Decision != tt.wantDec {
				t.Errorf("Decision = %q, want %q", parsed.Decision, tt.wantDec)
			}
		})
	}
}
