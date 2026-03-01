package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/runner/specmerge"
)

func TestPRSListDisplaysOpenPRs(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("mkdir gromit dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	fakeStates := []*specmerge.PRState{
		{
			SpecName: "spec-open",
			PRRef:    specmerge.PRRef{Number: 5},
		},
		{
			SpecName: "spec-closed",
			PRRef:    specmerge.PRRef{Number: 8},
		},
		{
			SpecName: "spec-no-pr",
		},
	}

	fakeStore := &fakePRStateStore{states: fakeStates}
	origStoreFn := newPRStateStoreFn
	origClientFn := newPRClientFn
	newPRStateStoreFn = func(gromitDir string) (specmerge.PRStateStore, error) {
		return fakeStore, nil
	}
	fakeClient := &fakePRClient{
		statuses: map[int]specmerge.PRStatus{
			5: {Number: 5, Title: "Add feature", State: "open"},
			8: {Number: 8, Title: "Old change", State: "closed"},
		},
	}
	newPRClientFn = func() specmerge.PRClient { return fakeClient }
	t.Cleanup(func() {
		newPRStateStoreFn = origStoreFn
		newPRClientFn = origClientFn
	})

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	stdout, stderr, exitCode := runGromitCobra(t, "prs")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d, stderr: %s", exitCode, stderr)
	}

	expected := "Open spec PRs (1):\n  spec-open  #5  Add feature\n"
	if stdout != expected {
		t.Fatalf("unexpected output:\nwant:\n%s\n\ngot:\n%s", expected, stdout)
	}
}

func TestPRSDetailShowsSpecPRInfo(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("mkdir gromit dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := `paths:
  gromit_dir: .gromit
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	detailState := &specmerge.PRState{
		SpecName:         "detail-spec",
		Outcome:          specmerge.PROutcomePending,
		AwaitingApproval: true,
		PRRef:            specmerge.PRRef{Number: 42},
		LastUpdated:      time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC),
		StageResults: []specmerge.StageResult{
			{
				StageName: "review-stage",
				Tier:      "fast",
				Passed:    true,
				ReviewResult: &review.ReviewResult{
					Summary: "All good",
				},
			},
		},
	}

	fakeStore := &fakePRStateStore{states: []*specmerge.PRState{detailState}}
	origStoreFn := newPRStateStoreFn
	origClientFn := newPRClientFn
	newPRStateStoreFn = func(gromitDir string) (specmerge.PRStateStore, error) {
		return fakeStore, nil
	}
	fakeClient := &fakePRClient{
		statuses: map[int]specmerge.PRStatus{
			42: {Number: 42, Title: "Improve detail output", State: "open"},
		},
		checks: map[int][]specmerge.CheckStatus{
			42: {
				{Name: "ci/build", Status: "in_progress", Conclusion: "pending"},
				{Name: "ci/test", Status: "completed", Conclusion: "success"},
			},
		},
	}
	newPRClientFn = func() specmerge.PRClient { return fakeClient }
	t.Cleanup(func() {
		newPRStateStoreFn = origStoreFn
		newPRClientFn = origClientFn
	})

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	stdout, stderr, exitCode := runGromitCobra(t, "prs", "detail-spec")
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d, stderr: %s", exitCode, stderr)
	}

	expected := "Spec: detail-spec\n" +
		"PR: #42\n" +
		"Title: Improve detail output\n" +
		"State: open\n" +
		"Outcome: pending\n" +
		"Awaiting approval: yes\n" +
		"Last updated: 2025-01-02T03:04:05Z\n" +
		"Stage results (1):\n" +
		"  review-stage (tier=fast) - passed=true - summary=All good\n" +
		"Checks (2):\n" +
		"  ci/build - status=in_progress - conclusion=pending\n" +
		"  ci/test - status=completed - conclusion=success\n"
	if stdout != expected {
		t.Fatalf("unexpected detail output:\nwant:\n%s\n\ngot:\n%s", expected, stdout)
	}
}

type fakePRStateStore struct {
	states []*specmerge.PRState
}

func (f *fakePRStateStore) List(context.Context) ([]*specmerge.PRState, error) {
	return f.states, nil
}

func (f *fakePRStateStore) Save(context.Context, *specmerge.PRState) error {
	return nil
}

type fakePRClient struct {
	statuses map[int]specmerge.PRStatus
	checks   map[int][]specmerge.CheckStatus
}

func (f *fakePRClient) CreatePR(context.Context, string, string, string, string) (specmerge.PRRef, error) {
	return specmerge.PRRef{}, nil
}

func (f *fakePRClient) GetPR(_ context.Context, ref specmerge.PRRef) (specmerge.PRStatus, error) {
	if status, ok := f.statuses[ref.Number]; ok {
		return status, nil
	}
	return specmerge.PRStatus{}, nil
}

func (f *fakePRClient) ListChecks(_ context.Context, ref specmerge.PRRef) ([]specmerge.CheckStatus, error) {
	if checks, ok := f.checks[ref.Number]; ok {
		return checks, nil
	}
	return nil, nil
}

func (f *fakePRClient) PostReview(context.Context, specmerge.PRRef, specmerge.ReviewPayload) error {
	return nil
}

func (f *fakePRClient) PostComment(context.Context, specmerge.PRRef, string) error {
	return nil
}

func (f *fakePRClient) RequestReviewers(context.Context, specmerge.PRRef, []string) error {
	return nil
}

func (f *fakePRClient) MergePR(context.Context, specmerge.PRRef, string) error {
	return nil
}
