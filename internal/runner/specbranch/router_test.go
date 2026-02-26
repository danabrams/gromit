package specbranch_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/runner/specbranch"
)

func TestRouterBranchForLabels(t *testing.T) {
	t.Parallel()

	router := specbranch.NewRouter()

	tests := []struct {
		name       string
		labels     []string
		wantBranch string
		wantErr    bool
	}{
		{
			name:       "non-spec defaults to main",
			labels:     nil,
			wantBranch: "main",
		},
		{
			name:       "spec branch",
			labels:     []string{"spec:auth"},
			wantBranch: "gromit/spec-auth",
		},
		{
			name:    "empty spec label",
			labels:  []string{"spec:"},
			wantErr: true,
		},
		{
			name:    "invalid spec name",
			labels:  []string{"spec:bad name"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch, err := router.BranchForLabels(tt.labels)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("BranchForLabels() = %q, want error", branch)
				}
				return
			}
			if err != nil {
				t.Fatalf("BranchForLabels() error = %v", err)
			}
			if branch != tt.wantBranch {
				t.Fatalf("BranchForLabels() = %q, want %q", branch, tt.wantBranch)
			}
		})
	}
}
