package specloop

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestValidateSubTasks(t *testing.T) {
	tests := []struct {
		name            string
		subTasks        []runstore.Task
		wantErr         bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "all valid objectives",
			subTasks: []runstore.Task{
				{TaskID: "t-005", Objective: "refactor parser"},
				{TaskID: "t-006", Objective: "add tests"},
			},
			wantErr: false,
		},
		{
			name:     "empty slice is valid",
			subTasks: []runstore.Task{},
			wantErr:  false,
		},
		{
			name: "single empty objective",
			subTasks: []runstore.Task{
				{TaskID: "t-005", Objective: ""},
			},
			wantErr:      true,
			wantContains: []string{"t-005"},
		},
		{
			name: "multiple empty objectives",
			subTasks: []runstore.Task{
				{TaskID: "t-005", Objective: ""},
				{TaskID: "t-006", Objective: ""},
			},
			wantErr:      true,
			wantContains: []string{"t-005", "t-006"},
		},
		{
			name: "whitespace-only objective",
			subTasks: []runstore.Task{
				{TaskID: "t-005", Objective: "   "},
			},
			wantErr:      true,
			wantContains: []string{"t-005"},
		},
		{
			name: "mixed valid and empty",
			subTasks: []runstore.Task{
				{TaskID: "t-005", Objective: "refactor parser"},
				{TaskID: "t-006", Objective: ""},
			},
			wantErr:         true,
			wantContains:    []string{"t-006"},
			wantNotContains: []string{"t-005"},
		},
		{
			name: "mixed valid and whitespace",
			subTasks: []runstore.Task{
				{TaskID: "t-005", Objective: "refactor parser"},
				{TaskID: "t-006", Objective: "  \t\n  "},
			},
			wantErr:      true,
			wantContains: []string{"t-006"},
		},
		{
			name: "all whitespace objectives",
			subTasks: []runstore.Task{
				{TaskID: "t-005", Objective: "  "},
				{TaskID: "t-006", Objective: "\t"},
				{TaskID: "t-007", Objective: "\n"},
			},
			wantErr:      true,
			wantContains: []string{"t-005", "t-006", "t-007"},
		},
		{
			name: "error message format includes offending IDs not valid",
			subTasks: []runstore.Task{
				{TaskID: "t-001", Objective: ""},
				{TaskID: "t-002", Objective: "valid objective"},
				{TaskID: "t-003", Objective: "   "},
			},
			wantErr:         true,
			wantContains:    []string{"t-001", "t-003"},
			wantNotContains: []string{"t-002"},
		},
		{
			name: "empty TaskID uses positional fallback",
			subTasks: []runstore.Task{
				{TaskID: "", Objective: ""},
			},
			wantErr:      true,
			wantContains: []string{"index=0"},
		},
		{
			name: "empty TaskID positional fallback at non-zero index",
			subTasks: []runstore.Task{
				{TaskID: "t-001", Objective: "valid"},
				{TaskID: "", Objective: ""},
			},
			wantErr:         true,
			wantContains:    []string{"index=1"},
			wantNotContains: []string{"t-001"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSubTasks(tc.subTasks)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if err != nil {
				msg := err.Error()
				for _, want := range tc.wantContains {
					if !strings.Contains(msg, want) {
						t.Errorf("error should contain %q, got: %v", want, msg)
					}
				}
				for _, notWant := range tc.wantNotContains {
					if strings.Contains(msg, notWant) {
						t.Errorf("error should NOT contain %q, got: %v", notWant, msg)
					}
				}
			}
		})
	}
}
