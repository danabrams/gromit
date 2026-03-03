package specbranch_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/runner/specbranch"
)

func TestRouterBranchForLabels(t *testing.T) {
	t.Parallel()

	router := specbranch.NewRouter("")

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

func TestRouterUsesConfiguredBaseBranch(t *testing.T) {
	t.Parallel()

	router := specbranch.NewRouter("develop")
	branch, err := router.BranchForLabels(nil)
	if err != nil {
		t.Fatalf("BranchForLabels() error = %v", err)
	}
	if branch != "develop" {
		t.Fatalf("BranchForLabels() = %q, want %q", branch, "develop")
	}
}

func TestRouterResolve(t *testing.T) {
	t.Parallel()

	router := specbranch.NewRouter("main")

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
			branch, err := router.Resolve(tt.labels)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Resolve() = %q, want error", branch)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if branch != tt.wantBranch {
				t.Fatalf("Resolve() = %q, want %q", branch, tt.wantBranch)
			}
		})
	}
}

func TestRouterSessionWorktreeSkipsBaseBranch(t *testing.T) {
	t.Parallel()

	router := specbranch.NewRouter("main")
	router.EnableSessionWorktreeMode()
	branch, err := router.BranchForLabels(nil)
	if err != nil {
		t.Fatalf("BranchForLabels() error = %v", err)
	}
	if branch != "" {
		t.Fatalf("BranchForLabels() = %q, want empty string in session worktree mode", branch)
	}

	specBranch, err := router.BranchForLabels([]string{"spec:auth"})
	if err != nil {
		t.Fatalf("BranchForLabels() error = %v", err)
	}
	if specBranch != "gromit/spec-auth" {
		t.Fatalf("BranchForLabels() = %q, want %q", specBranch, "gromit/spec-auth")
	}
}

func TestHasSpecLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		labels []string
		want   bool
	}{
		{
			name:   "no labels",
			labels: nil,
			want:   false,
		},
		{
			name:   "non-spec labels",
			labels: []string{"bug", "from-review"},
			want:   false,
		},
		{
			name:   "spec label present",
			labels: []string{"from-review", "spec:auth"},
			want:   true,
		},
		{
			name:   "empty spec label still treated as spec-scoped",
			labels: []string{"spec:"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := specbranch.HasSpecLabel(tt.labels)
			if got != tt.want {
				t.Fatalf("HasSpecLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}
