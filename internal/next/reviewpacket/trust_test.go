package reviewpacket

import "testing"

func TestComputeTrustLevel(t *testing.T) {
	tests := []struct {
		name                     string
		terminalState            string
		validationPassed         bool
		acceptanceAllPassed      bool
		hasBlockingFindings      bool
		hasDegradedFlags         bool
		repeatedFailureEscalated bool
		expectedTrust            string
	}{
		{
			name:                     "high trust: clean run",
			terminalState:            "ready_for_review",
			validationPassed:         true,
			acceptanceAllPassed:      true,
			hasBlockingFindings:      false,
			hasDegradedFlags:         false,
			repeatedFailureEscalated: false,
			expectedTrust:            "high",
		},
		{
			name:                     "high trust: ready_for_review with all signals clean",
			terminalState:            "ready_for_review",
			validationPassed:         true,
			acceptanceAllPassed:      true,
			hasBlockingFindings:      false,
			hasDegradedFlags:         false,
			repeatedFailureEscalated: false,
			expectedTrust:            "high",
		},
		{
			name:                     "medium trust: degraded flags present but validation passed and no blockers",
			terminalState:            "ready_for_review",
			validationPassed:         true,
			acceptanceAllPassed:      true,
			hasBlockingFindings:      false,
			hasDegradedFlags:         true,
			repeatedFailureEscalated: false,
			expectedTrust:            "medium",
		},
		{
			name:                     "medium trust: non-blocking findings but no blockers",
			terminalState:            "ready_for_review",
			validationPassed:         true,
			acceptanceAllPassed:      true,
			hasBlockingFindings:      false,
			hasDegradedFlags:         false,
			repeatedFailureEscalated: false,
			expectedTrust:            "high",
		},
		{
			name:                     "low trust: run in blocked state",
			terminalState:            "blocked",
			validationPassed:         false,
			acceptanceAllPassed:      false,
			hasBlockingFindings:      false,
			hasDegradedFlags:         false,
			repeatedFailureEscalated: false,
			expectedTrust:            "low",
		},
		{
			name:                     "low trust: run in needs_human state",
			terminalState:            "needs_human",
			validationPassed:         false,
			acceptanceAllPassed:      false,
			hasBlockingFindings:      false,
			hasDegradedFlags:         false,
			repeatedFailureEscalated: false,
			expectedTrust:            "low",
		},
		{
			name:                     "low trust: validation failed",
			terminalState:            "ready_for_review",
			validationPassed:         false,
			acceptanceAllPassed:      true,
			hasBlockingFindings:      false,
			hasDegradedFlags:         false,
			repeatedFailureEscalated: false,
			expectedTrust:            "low",
		},
		{
			name:                     "low trust: has blocking findings",
			terminalState:            "ready_for_review",
			validationPassed:         true,
			acceptanceAllPassed:      true,
			hasBlockingFindings:      true,
			hasDegradedFlags:         false,
			repeatedFailureEscalated: false,
			expectedTrust:            "low",
		},
		{
			name:                     "low trust: repeated failure escalation",
			terminalState:            "ready_for_review",
			validationPassed:         true,
			acceptanceAllPassed:      true,
			hasBlockingFindings:      false,
			hasDegradedFlags:         false,
			repeatedFailureEscalated: true,
			expectedTrust:            "low",
		},
		{
			name:                     "low trust: acceptance not all passed",
			terminalState:            "ready_for_review",
			validationPassed:         true,
			acceptanceAllPassed:      false,
			hasBlockingFindings:      false,
			hasDegradedFlags:         false,
			repeatedFailureEscalated: false,
			expectedTrust:            "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeTrustLevel(
				tt.terminalState,
				tt.validationPassed,
				tt.acceptanceAllPassed,
				tt.hasBlockingFindings,
				tt.hasDegradedFlags,
				tt.repeatedFailureEscalated,
			)
			if got != tt.expectedTrust {
				t.Errorf("ComputeTrustLevel() = %q, want %q", got, tt.expectedTrust)
			}
		})
	}
}

func TestRecommendedPosture(t *testing.T) {
	tests := []struct {
		name            string
		trustLevel      string
		expectedPosture string
	}{
		{
			name:            "high trust maps to quick_accept_path",
			trustLevel:      "high",
			expectedPosture: "quick_accept_path",
		},
		{
			name:            "medium trust maps to manual_check_carefully",
			trustLevel:      "medium",
			expectedPosture: "manual_check_carefully",
		},
		{
			name:            "low trust maps to do_not_accept_without_changes",
			trustLevel:      "low",
			expectedPosture: "do_not_accept_without_changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecommendedPosture(tt.trustLevel)
			if got != tt.expectedPosture {
				t.Errorf("RecommendedPosture(%q) = %q, want %q", tt.trustLevel, got, tt.expectedPosture)
			}
		})
	}
}
