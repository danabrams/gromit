package specmerge

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// PROutcome describes the high-level state of a spec PR between iterations.
type PROutcome string

const (
	PROutcomeUnknown          PROutcome = ""
	PROutcomePending          PROutcome = "pending"
	PROutcomeApproved         PROutcome = "approved"
	PROutcomeChangesRequested PROutcome = "changes_requested"
	PROutcomeClosed           PROutcome = "closed"
	PROutcomeMerged           PROutcome = "merged"
)

// PRState tracks the live status of a spec PR so the orchestrator can react
// to merged/approved/changes-requested/closed/pending outcomes.
type PRState struct {
	SpecName         string        `json:"spec_name"`
	PRRef            PRRef         `json:"pr_ref"`
	Outcome          PROutcome     `json:"outcome"`
	AwaitingApproval bool          `json:"awaiting_approval"`
	LastChecks       []CheckStatus `json:"last_checks,omitempty"`
	LastUpdated      time.Time     `json:"last_updated"`
}

// PRStateStore holds PRState entries for all active spec PRs.
type PRStateStore interface {
	List(ctx context.Context) ([]*PRState, error)
	Save(ctx context.Context, state *PRState) error
}

// Poller inspects live PR data and updates PRState entries accordingly.
type Poller struct {
	client PRClient
	store  PRStateStore
}

// NewPoller constructs a Poller backed by a PR client and a PR state store.
func NewPoller(client PRClient, store PRStateStore) *Poller {
	return &Poller{client: client, store: store}
}

// Poll refreshes every non-terminal PRState in the store with the latest GitHub data.
func (p *Poller) Poll(ctx context.Context) error {
	if p == nil {
		return fmt.Errorf("poller is nil")
	}
	if p.client == nil {
		return fmt.Errorf("pr client is required")
	}
	if p.store == nil {
		return fmt.Errorf("state store is required")
	}

	states, err := p.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list pr states: %w", err)
	}

	for _, state := range states {
		if state == nil || state.PRRef.Number == 0 {
			continue
		}
		if state.Outcome == PROutcomeMerged || state.Outcome == PROutcomeClosed {
			continue
		}

		prStatus, err := p.client.GetPR(ctx, state.PRRef)
		if err != nil {
			return fmt.Errorf("get pr %d: %w", state.PRRef.Number, err)
		}

		checks, err := p.client.ListChecks(ctx, state.PRRef)
		if err != nil {
			return fmt.Errorf("list checks for pr %d: %w", state.PRRef.Number, err)
		}

		state.LastChecks = append([]CheckStatus(nil), checks...)
		state.Outcome = deriveOutcome(prStatus, checks)
		state.AwaitingApproval = state.Outcome == PROutcomeApproved
		state.LastUpdated = time.Now()

		if err := p.store.Save(ctx, state); err != nil {
			return fmt.Errorf("save state for spec %q: %w", state.SpecName, err)
		}
	}

	return nil
}

func deriveOutcome(status PRStatus, checks []CheckStatus) PROutcome {
	switch strings.ToLower(strings.TrimSpace(status.State)) {
	case "merged":
		return PROutcomeMerged
	case "closed":
		return PROutcomeClosed
	}

	if hasPendingChecks(checks) || len(checks) == 0 {
		return PROutcomePending
	}

	if hasFailedChecks(checks) {
		return PROutcomeChangesRequested
	}

	return PROutcomeApproved
}

func hasPendingChecks(checks []CheckStatus) bool {
	for _, check := range checks {
		if isPendingStatus(check.Status) {
			return true
		}
	}
	return false
}

func hasFailedChecks(checks []CheckStatus) bool {
	for _, check := range checks {
		if isFailureConclusion(check.Conclusion) {
			return true
		}
	}
	return false
}

func isPendingStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "in_progress", "queued":
		return true
	}
	return false
}

func isFailureConclusion(conclusion string) bool {
	switch strings.ToLower(strings.TrimSpace(conclusion)) {
	case "failure", "cancelled":
		return true
	}
	return false
}
