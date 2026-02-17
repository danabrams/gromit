package andon

import (
	"testing"
	"time"
)

// Expected failure: hard-stop failure kinds and decision path constants are not
// implemented yet, so policy still routes through existing autonomous L1/L2
// retry behavior.
func TestEvaluateFailureWithTrace_HardStopActionsBypassAutonomousL1L2(t *testing.T) {
	now := time.Date(2026, time.February, 16, 11, 0, 0, 0, time.UTC)
	thresholds := DefaultThresholds()

	tests := []struct {
		name   string
		signal FailureSignal
	}{
		{
			name:   "bulk delete outside scoped tmp dirs",
			signal: FailureSignal{Kind: FailureKindHardStopBulkDelete, Output: "rm -rf /var/lib/postgres"},
		},
		{
			name:   "irreversible migration",
			signal: FailureSignal{Kind: FailureKindHardStopIrreversibleMigration, Output: "alembic upgrade head --irreversible"},
		},
		{
			name:   "credential or secret mutation",
			signal: FailureSignal{Kind: FailureKindHardStopCredentialChange, Output: "rotate production db password"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := EvaluateFailureWithTrace(
				tt.signal,
				RecoveryState{
					Level:      LevelL1,
					L1Attempts: 0,
					L1Started:  now,
				},
				thresholds,
				now,
			)

			if trace.Decision != (PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate}) {
				t.Fatalf("EvaluateFailureWithTrace(...).Decision = %+v, want L3 escalation for hard-stop", trace.Decision)
			}
			if trace.Path != DecisionPathHardStopRequiresApproval {
				t.Fatalf("EvaluateFailureWithTrace(...).Path = %q, want %q", trace.Path, DecisionPathHardStopRequiresApproval)
			}
		})
	}
}

// Expected failure: HardStopContext and allowlist-aware bulk delete classification
// are not implemented yet, so scoped tmp deletions are not explicitly allowlisted.
func TestEvaluateFailureWithTrace_BulkDeleteAllowlistIsExplicit(t *testing.T) {
	now := time.Date(2026, time.February, 16, 11, 30, 0, 0, time.UTC)
	thresholds := DefaultThresholds()

	tests := []struct {
		name     string
		signal   FailureSignal
		want     PolicyDecision
		wantPath DecisionPath
	}{
		{
			name: "delete inside allowed tmp dir stays autonomous",
			signal: FailureSignal{
				Kind: FailureKindHardStopBulkDelete,
				HardStop: HardStopContext{
					Command:             "rm -rf /tmp/gromit/workdir-123",
					BulkDeleteAllowlist: []string{"/tmp/gromit"},
				},
			},
			want:     PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry},
			wantPath: DecisionPathTransientL1Retry,
		},
		{
			name: "delete outside allowlist requires escalation",
			signal: FailureSignal{
				Kind: FailureKindHardStopBulkDelete,
				HardStop: HardStopContext{
					Command:             "rm -rf /home/dabrams/gromit/internal",
					BulkDeleteAllowlist: []string{"/tmp/gromit"},
				},
			},
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate},
			wantPath: DecisionPathHardStopBulkDeleteOutsideAllowlist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := EvaluateFailureWithTrace(
				tt.signal,
				RecoveryState{
					Level:      LevelL1,
					L1Attempts: 0,
					L1Started:  now,
				},
				thresholds,
				now,
			)

			if trace.Decision != tt.want {
				t.Fatalf("EvaluateFailureWithTrace(...).Decision = %+v, want %+v", trace.Decision, tt.want)
			}
			if trace.Path != tt.wantPath {
				t.Fatalf("EvaluateFailureWithTrace(...).Path = %q, want %q", trace.Path, tt.wantPath)
			}
		})
	}
}
