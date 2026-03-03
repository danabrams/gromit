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

func TestFakePRClient_TracksCalls(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fake := &fakePRClient{}

	ref := specmerge.PRRef{Owner: "owner", Repo: "repo", Number: 1}

	// Verify PostReview call is tracked
	payload := specmerge.ReviewPayload{Event: "APPROVE", Body: "looks good"}
	err := fake.PostReview(ctx, ref, payload)
	if err != nil {
		t.Fatalf("PostReview returned error: %v", err)
	}
	if len(fake.postReviewCalls) == 0 {
		t.Error("PostReview should track calls")
	}
	if len(fake.postReviewCalls) > 0 && fake.postReviewCalls[0].Event != "APPROVE" {
		t.Errorf("PostReview call event was %q, want APPROVE", fake.postReviewCalls[0].Event)
	}

	// Verify PostComment call is tracked
	err = fake.PostComment(ctx, ref, "comment body")
	if err != nil {
		t.Fatalf("PostComment returned error: %v", err)
	}
	if len(fake.postCommentCalls) == 0 {
		t.Error("PostComment should track calls")
	}

	// Verify RequestReviewers call is tracked
	err = fake.RequestReviewers(ctx, ref, []string{"reviewer1", "reviewer2"})
	if err != nil {
		t.Fatalf("RequestReviewers returned error: %v", err)
	}
	if len(fake.requestReviewersCalls) == 0 {
		t.Error("RequestReviewers should track calls")
	}
	if len(fake.requestReviewersCalls) > 0 && len(fake.requestReviewersCalls[0]) != 2 {
		t.Errorf("RequestReviewers tracked %d reviewers, want 2", len(fake.requestReviewersCalls[0]))
	}

	// Verify MergePR call is tracked
	err = fake.MergePR(ctx, ref, "merge message")
	if err != nil {
		t.Fatalf("MergePR returned error: %v", err)
	}
	if len(fake.mergePRCalls) == 0 {
		t.Error("MergePR should track calls")
	}
}

func TestFakePRClient_ReturnsErrorsForAllMethods(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testError := errors.New("API error")

	tests := map[string]func(*fakePRClient) error{
		"GetPR": func(f *fakePRClient) error {
			_, err := f.GetPR(ctx, specmerge.PRRef{Number: 1})
			return err
		},
		"ListChecks": func(f *fakePRClient) error {
			_, err := f.ListChecks(ctx, specmerge.PRRef{Number: 1})
			return err
		},
		"PostReview": func(f *fakePRClient) error {
			return f.PostReview(ctx, specmerge.PRRef{Number: 1}, specmerge.ReviewPayload{})
		},
		"PostComment": func(f *fakePRClient) error {
			return f.PostComment(ctx, specmerge.PRRef{Number: 1}, "body")
		},
		"RequestReviewers": func(f *fakePRClient) error {
			return f.RequestReviewers(ctx, specmerge.PRRef{Number: 1}, []string{})
		},
		"MergePR": func(f *fakePRClient) error {
			return f.MergePR(ctx, specmerge.PRRef{Number: 1}, "message")
		},
	}

	for methodName, testFn := range tests {
		t.Run(methodName, func(t *testing.T) {
			fake := &fakePRClient{
				getPRError:            testError,
				listChecksError:       testError,
				postReviewError:       testError,
				postCommentError:      testError,
				requestReviewersError: testError,
				mergePRError:          testError,
			}

			err := testFn(fake)
			if err == nil {
				t.Errorf("%s should return configured error", methodName)
			}
			if err.Error() != testError.Error() {
				t.Errorf("%s returned wrong error: %v", methodName, err)
			}
		})
	}
}

func TestPRRef_StructFields(t *testing.T) {
	t.Parallel()

	ref := specmerge.PRRef{
		Owner:  "octocat",
		Repo:   "Hello-World",
		Number: 1347,
	}

	if ref.Owner != "octocat" {
		t.Errorf("PRRef.Owner = %q, want %q", ref.Owner, "octocat")
	}
	if ref.Repo != "Hello-World" {
		t.Errorf("PRRef.Repo = %q, want %q", ref.Repo, "Hello-World")
	}
	if ref.Number != 1347 {
		t.Errorf("PRRef.Number = %d, want %d", ref.Number, 1347)
	}
}

func TestPRStatus_StructFields(t *testing.T) {
	t.Parallel()

	status := specmerge.PRStatus{
		Number:    1,
		Title:     "Amazing Feature",
		State:     "open",
		IsDraft:   true,
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "2025-01-02T00:00:00Z",
	}

	if status.Number != 1 {
		t.Errorf("PRStatus.Number = %d, want %d", status.Number, 1)
	}
	if status.Title != "Amazing Feature" {
		t.Errorf("PRStatus.Title = %q, want %q", status.Title, "Amazing Feature")
	}
	if status.State != "open" {
		t.Errorf("PRStatus.State = %q, want %q", status.State, "open")
	}
	if status.IsDraft != true {
		t.Errorf("PRStatus.IsDraft = %v, want %v", status.IsDraft, true)
	}
}

func TestCheckStatus_StructFields(t *testing.T) {
	t.Parallel()

	check := specmerge.CheckStatus{
		Name:       "ci/lint",
		Status:     "completed",
		Conclusion: "success",
		DetailsURL: "https://example.com/checks/1",
	}

	if check.Name != "ci/lint" {
		t.Errorf("CheckStatus.Name = %q, want %q", check.Name, "ci/lint")
	}
	if check.Status != "completed" {
		t.Errorf("CheckStatus.Status = %q, want %q", check.Status, "completed")
	}
	if check.Conclusion != "success" {
		t.Errorf("CheckStatus.Conclusion = %q, want %q", check.Conclusion, "success")
	}
}

func TestReviewPayload_WithComments(t *testing.T) {
	t.Parallel()

	payload := specmerge.ReviewPayload{
		Event: "REQUEST_CHANGES",
		Body:  "Please update the code",
		Comments: []specmerge.ReviewComment{
			{
				Path: "main.go",
				Line: 42,
				Body: "This line has an issue",
			},
			{
				Path: "main_test.go",
				Line: 10,
				Body: "Missing test case",
			},
		},
	}

	if payload.Event != "REQUEST_CHANGES" {
		t.Errorf("ReviewPayload.Event = %q, want %q", payload.Event, "REQUEST_CHANGES")
	}
	if len(payload.Comments) != 2 {
		t.Errorf("ReviewPayload.Comments length = %d, want 2", len(payload.Comments))
	}
	if payload.Comments[0].Path != "main.go" {
		t.Errorf("First comment path = %q, want main.go", payload.Comments[0].Path)
	}
	if payload.Comments[0].Line != 42 {
		t.Errorf("First comment line = %d, want 42", payload.Comments[0].Line)
	}
}

// fakePRClient is a test implementation of PRClient interface.
type fakePRClient struct {
	nextPRNumber          int
	createPRError         error
	createPRFn            func(ctx context.Context, title, body, head, base string) (specmerge.PRRef, error)
	getPRError            error
	listChecksError       error
	postReviewError       error
	postCommentError      error
	requestReviewersError error
	mergePRError          error
	checksToReturn        []specmerge.CheckStatus
	postReviewCalls       []specmerge.ReviewPayload
	postCommentCalls      []string
	requestReviewersCalls [][]string
	mergePRCalls          []string
}

func (f *fakePRClient) CreatePR(ctx context.Context, title, body, head, base string) (specmerge.PRRef, error) {
	if f.createPRFn != nil {
		return f.createPRFn(ctx, title, body, head, base)
	}
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
	if f.getPRError != nil {
		return specmerge.PRStatus{}, f.getPRError
	}
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
	if f.listChecksError != nil {
		return nil, f.listChecksError
	}
	if f.checksToReturn != nil {
		return f.checksToReturn, nil
	}
	return []specmerge.CheckStatus{}, nil
}

func (f *fakePRClient) PostReview(ctx context.Context, ref specmerge.PRRef, payload specmerge.ReviewPayload) error {
	if f.postReviewError != nil {
		return f.postReviewError
	}
	f.postReviewCalls = append(f.postReviewCalls, payload)
	return nil
}

func (f *fakePRClient) PostComment(ctx context.Context, ref specmerge.PRRef, body string) error {
	if f.postCommentError != nil {
		return f.postCommentError
	}
	f.postCommentCalls = append(f.postCommentCalls, body)
	return nil
}

func (f *fakePRClient) RequestReviewers(ctx context.Context, ref specmerge.PRRef, reviewers []string) error {
	if f.requestReviewersError != nil {
		return f.requestReviewersError
	}
	f.requestReviewersCalls = append(f.requestReviewersCalls, reviewers)
	return nil
}

func (f *fakePRClient) MergePR(ctx context.Context, ref specmerge.PRRef, commitMessage string) error {
	if f.mergePRError != nil {
		return f.mergePRError
	}
	f.mergePRCalls = append(f.mergePRCalls, commitMessage)
	return nil
}
