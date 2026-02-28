package specmerge_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/runner/specmerge"
)

func TestPRClient_InterfaceExists(t *testing.T) {
	t.Parallel()

	// Verify PRClient interface exists and has all required methods
	var _ specmerge.PRClient

	// Verify interface has the required methods by checking method signatures
	iface := reflect.TypeOf((*specmerge.PRClient)(nil)).Elem()

	requiredMethods := map[string]bool{
		"CreatePR":         false,
		"GetPR":            false,
		"ListChecks":       false,
		"PostReview":       false,
		"PostComment":      false,
		"RequestReviewers": false,
		"MergePR":          false,
	}

	for i := 0; i < iface.NumMethod(); i++ {
		method := iface.Method(i)
		if _, ok := requiredMethods[method.Name]; ok {
			requiredMethods[method.Name] = true
		}
	}

	for method, found := range requiredMethods {
		if !found {
			t.Errorf("PRClient missing method: %s", method)
		}
	}
}

func TestFakePRClient_ImplementsPRClient(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fake := &fakePRClient{}

	// Verify it can be assigned to PRClient interface
	var client specmerge.PRClient = fake
	if client == nil {
		t.Fatal("fakePRClient does not implement PRClient")
	}

	// Test CreatePR
	ref, err := fake.CreatePR(ctx, "title", "body", "feature", "main")
	if err != nil {
		t.Fatalf("CreatePR returned error: %v", err)
	}
	if ref.Number == 0 {
		t.Error("CreatePR should return a PR number")
	}

	// Test GetPR
	status, err := fake.GetPR(ctx, ref)
	if err != nil {
		t.Fatalf("GetPR returned error: %v", err)
	}
	if status.Number != ref.Number {
		t.Errorf("GetPR returned wrong PR number: %d, want %d", status.Number, ref.Number)
	}

	// Test ListChecks
	checks, err := fake.ListChecks(ctx, ref)
	if err != nil {
		t.Fatalf("ListChecks returned error: %v", err)
	}
	if checks == nil {
		t.Error("ListChecks should return a slice, not nil")
	}

	// Test PostReview
	payload := specmerge.ReviewPayload{Event: "APPROVE", Body: "looks good"}
	err = fake.PostReview(ctx, ref, payload)
	if err != nil {
		t.Fatalf("PostReview returned error: %v", err)
	}

	// Test PostComment
	err = fake.PostComment(ctx, ref, "comment body")
	if err != nil {
		t.Fatalf("PostComment returned error: %v", err)
	}

	// Test RequestReviewers
	err = fake.RequestReviewers(ctx, ref, []string{"reviewer1"})
	if err != nil {
		t.Fatalf("RequestReviewers returned error: %v", err)
	}

	// Test MergePR
	err = fake.MergePR(ctx, ref, "merge commit message")
	if err != nil {
		t.Fatalf("MergePR returned error: %v", err)
	}
}

func TestFakePRClient_TracksCreatePRCalls(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fake := &fakePRClient{}

	ref1, _ := fake.CreatePR(ctx, "title1", "body1", "feature1", "main")
	ref2, _ := fake.CreatePR(ctx, "title2", "body2", "feature2", "main")

	if ref1.Number == ref2.Number {
		t.Error("CreatePR should return unique PR numbers")
	}
}

func TestFakePRClient_CanReturnErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fake := &fakePRClient{
		createPRError: errors.New("network error"),
	}

	_, err := fake.CreatePR(ctx, "title", "body", "feature", "main")
	if err == nil {
		t.Error("CreatePR should return configured error")
	}
}

func TestFakePRClient_ListChecks_ReturnsConfiguredChecks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checks := []specmerge.CheckStatus{
		{
			Name:       "ci/test",
			Status:     "completed",
			Conclusion: "success",
		},
		{
			Name:       "ci/lint",
			Status:     "completed",
			Conclusion: "failure",
		},
	}
	fake := &fakePRClient{
		checksToReturn: checks,
	}

	ref := specmerge.PRRef{Owner: "owner", Repo: "repo", Number: 1}
	result, err := fake.ListChecks(ctx, ref)
	if err != nil {
		t.Fatalf("ListChecks returned error: %v", err)
	}
	if len(result) != len(checks) {
		t.Errorf("ListChecks returned %d checks, want %d", len(result), len(checks))
	}
}

func TestFakePRClient_GetPR_ReturnsStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fake := &fakePRClient{}

	ref := specmerge.PRRef{Owner: "owner", Repo: "repo", Number: 1}
	status, err := fake.GetPR(ctx, ref)
	if err != nil {
		t.Fatalf("GetPR returned error: %v", err)
	}
	if status.Number != ref.Number {
		t.Errorf("GetPR returned number %d, want %d", status.Number, ref.Number)
	}
	if status.State == "" {
		t.Error("GetPR should return a non-empty state")
	}
}

// fakePRClient is a test implementation of PRClient interface.
type fakePRClient struct {
	nextPRNumber  int
	createPRError error
	checksToReturn []specmerge.CheckStatus
}

func (f *fakePRClient) CreatePR(ctx context.Context, title, body, head, base string) (specmerge.PRRef, error) {
	if f.createPRError != nil {
		return specmerge.PRRef{}, f.createPRError
	}
	f.nextPRNumber++
	return specmerge.PRRef{
		Owner:  "test-owner",
		Repo:   "test-repo",
		Number: f.nextPRNumber,
	}, nil
}

func (f *fakePRClient) GetPR(ctx context.Context, ref specmerge.PRRef) (specmerge.PRStatus, error) {
	return specmerge.PRStatus{
		Number:    ref.Number,
		Title:     "Test PR",
		State:     "open",
		IsDraft:   false,
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "2025-01-01T00:00:00Z",
	}, nil
}

func (f *fakePRClient) ListChecks(ctx context.Context, ref specmerge.PRRef) ([]specmerge.CheckStatus, error) {
	if f.checksToReturn != nil {
		return f.checksToReturn, nil
	}
	return []specmerge.CheckStatus{}, nil
}

func (f *fakePRClient) PostReview(ctx context.Context, ref specmerge.PRRef, payload specmerge.ReviewPayload) error {
	return nil
}

func (f *fakePRClient) PostComment(ctx context.Context, ref specmerge.PRRef, body string) error {
	return nil
}

func (f *fakePRClient) RequestReviewers(ctx context.Context, ref specmerge.PRRef, reviewers []string) error {
	return nil
}

func (f *fakePRClient) MergePR(ctx context.Context, ref specmerge.PRRef, commitMessage string) error {
	return nil
}
