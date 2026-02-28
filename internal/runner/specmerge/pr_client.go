package specmerge

import "context"

// PRRef identifies a pull request.
type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

// PRStatus describes the current state of a pull request.
type PRStatus struct {
	Number    int
	Title     string
	State     string // "open", "closed", "merged"
	IsDraft   bool
	CreatedAt string
	UpdatedAt string
}

// CheckStatus represents the status of a CI check.
type CheckStatus struct {
	Name       string
	Status     string // "pending", "in_progress", "completed"
	Conclusion string // "success", "failure", "neutral", "cancelled"
	DetailsURL string
}

// ReviewPayload contains the parameters for posting a review.
type ReviewPayload struct {
	Event   string // "APPROVE", "REQUEST_CHANGES", "COMMENT"
	Body    string
	Comments []ReviewComment
}

// ReviewComment represents a single comment in a review.
type ReviewComment struct {
	Path     string
	Line     int
	Body     string
}

// PRClient defines the interface for pull request operations.
type PRClient interface {
	// CreatePR creates a new pull request and returns its reference.
	CreatePR(ctx context.Context, title, body, head, base string) (PRRef, error)

	// GetPR retrieves the current status of a pull request.
	GetPR(ctx context.Context, ref PRRef) (PRStatus, error)

	// ListChecks lists all CI checks for a pull request.
	ListChecks(ctx context.Context, ref PRRef) ([]CheckStatus, error)

	// PostReview posts a review to a pull request.
	PostReview(ctx context.Context, ref PRRef, payload ReviewPayload) error

	// PostComment posts a comment to a pull request.
	PostComment(ctx context.Context, ref PRRef, body string) error

	// RequestReviewers requests reviewers for a pull request.
	RequestReviewers(ctx context.Context, ref PRRef, reviewers []string) error

	// MergePR merges a pull request.
	MergePR(ctx context.Context, ref PRRef, commitMessage string) error
}
