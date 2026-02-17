package andon

import (
	"strings"
	"testing"
)

func validEscalationPacket() EscalationPacket {
	return EscalationPacket{
		FailedCommand: "go test ./...",
		ErrorExcerpt:  "--- FAIL: TestRunner (0.02s)",
		L1Attempts: []EscalationAttempt{
			{Summary: "rerun validation", Outcome: "still failing"},
		},
		L2Attempts: []EscalationAttempt{
			{Summary: "escalate model tier", Outcome: "quality gate still failing"},
		},
		StateSnapshot: "bead=gromit-vd7f level=L2 retries=2",
		RiskLevel:     RiskLevelHigh,
		Options: []EscalationOption{
			{Title: "Retry L2", Tradeoff: "fastest path, but may repeat failure"},
			{Title: "Decompose bead", Tradeoff: "reduces risk, but slows delivery"},
			{Title: "Escalate to L3", Tradeoff: "max safety, but requires human intervention"},
		},
		Recommendation: "Decompose bead",
	}
}

func TestEscalationPacket_ValidateRequiresAllFields(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*EscalationPacket)
		wantErrSub string
	}{
		{
			name: "missing failed command",
			mutate: func(p *EscalationPacket) {
				p.FailedCommand = ""
			},
			wantErrSub: "failed_command",
		},
		{
			name: "missing exact error excerpt",
			mutate: func(p *EscalationPacket) {
				p.ErrorExcerpt = ""
			},
			wantErrSub: "error_excerpt",
		},
		{
			name: "missing l1 attempts",
			mutate: func(p *EscalationPacket) {
				p.L1Attempts = nil
			},
			wantErrSub: "l1_attempts",
		},
		{
			name: "missing l2 attempts",
			mutate: func(p *EscalationPacket) {
				p.L2Attempts = nil
			},
			wantErrSub: "l2_attempts",
		},
		{
			name: "missing state snapshot",
			mutate: func(p *EscalationPacket) {
				p.StateSnapshot = ""
			},
			wantErrSub: "state_snapshot",
		},
		{
			name: "missing risk level",
			mutate: func(p *EscalationPacket) {
				p.RiskLevel = ""
			},
			wantErrSub: "risk_level",
		},
		{
			name: "missing recommendation",
			mutate: func(p *EscalationPacket) {
				p.Recommendation = ""
			},
			wantErrSub: "recommendation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := validEscalationPacket()
			tt.mutate(&packet)

			err := ValidateEscalationPacket(packet)
			if err == nil {
				t.Fatalf("ValidateEscalationPacket() error = nil, want error containing %q", tt.wantErrSub)
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Fatalf("ValidateEscalationPacket() error = %q, want substring %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestEscalationPacket_FormatEnforcesThreeOptionsAndRecommendation(t *testing.T) {
	tests := []struct {
		name       string
		options    []EscalationOption
		rec        string
		wantErrSub string
	}{
		{
			name: "too few options",
			options: []EscalationOption{
				{Title: "Retry L2", Tradeoff: "fastest path, but may repeat failure"},
				{Title: "Decompose bead", Tradeoff: "reduces risk, but slows delivery"},
			},
			rec:        "Retry L2",
			wantErrSub: "exactly three options",
		},
		{
			name: "too many options",
			options: []EscalationOption{
				{Title: "Retry L2", Tradeoff: "fastest path, but may repeat failure"},
				{Title: "Decompose bead", Tradeoff: "reduces risk, but slows delivery"},
				{Title: "Escalate to L3", Tradeoff: "max safety, but requires human intervention"},
				{Title: "Pause run", Tradeoff: "avoids bad changes, but halts progress"},
			},
			rec:        "Escalate to L3",
			wantErrSub: "exactly three options",
		},
		{
			name: "missing recommendation",
			options: []EscalationOption{
				{Title: "Retry L2", Tradeoff: "fastest path, but may repeat failure"},
				{Title: "Decompose bead", Tradeoff: "reduces risk, but slows delivery"},
				{Title: "Escalate to L3", Tradeoff: "max safety, but requires human intervention"},
			},
			rec:        "",
			wantErrSub: "recommendation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet := validEscalationPacket()
			packet.Options = tt.options
			packet.Recommendation = tt.rec

			_, err := FormatEscalationPacket(packet)
			if err == nil {
				t.Fatalf("FormatEscalationPacket() error = nil, want error containing %q", tt.wantErrSub)
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Fatalf("FormatEscalationPacket() error = %q, want substring %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

func TestEscalationPacket_FormatIncludesThreeTradeoffsAndExplicitRecommendation(t *testing.T) {
	packet := validEscalationPacket()

	output, err := FormatEscalationPacket(packet)
	if err != nil {
		t.Fatalf("FormatEscalationPacket() error = %v, want nil", err)
	}

	if got := strings.Count(output, "Tradeoff:"); got != 3 {
		t.Fatalf("formatted output Tradeoff count = %d, want 3", got)
	}
	if !strings.Contains(output, "Recommendation:") {
		t.Fatalf("formatted output missing explicit recommendation section: %q", output)
	}
	if !strings.Contains(output, packet.Recommendation) {
		t.Fatalf("formatted output missing recommendation value %q: %q", packet.Recommendation, output)
	}
	if !strings.Contains(output, packet.FailedCommand) {
		t.Fatalf("formatted output missing failed command %q: %q", packet.FailedCommand, output)
	}
	if !strings.Contains(output, packet.ErrorExcerpt) {
		t.Fatalf("formatted output missing error excerpt %q: %q", packet.ErrorExcerpt, output)
	}
	if !strings.Contains(output, packet.StateSnapshot) {
		t.Fatalf("formatted output missing state snapshot %q: %q", packet.StateSnapshot, output)
	}
	if !strings.Contains(output, string(packet.RiskLevel)) {
		t.Fatalf("formatted output missing risk level %q: %q", packet.RiskLevel, output)
	}
	if !strings.Contains(output, packet.L1Attempts[0].Summary) {
		t.Fatalf("formatted output missing L1 attempt summary %q: %q", packet.L1Attempts[0].Summary, output)
	}
	if !strings.Contains(output, packet.L2Attempts[0].Summary) {
		t.Fatalf("formatted output missing L2 attempt summary %q: %q", packet.L2Attempts[0].Summary, output)
	}
}
