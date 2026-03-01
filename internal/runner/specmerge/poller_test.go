package specmerge

import (
    "context"
    "testing"
)

func TestPollerTransitions(t *testing.T) {
    tests := []struct {
        name          string
        prStatus      PRStatus
        checks        []CheckStatus
        wantOutcome   PROutcome
        wantAwaiting  bool
    }{
        {
            name:        "merged",
            prStatus:    PRStatus{State: "merged"},
            wantOutcome: PROutcomeMerged,
        },
        {
            name:        "closed",
            prStatus:    PRStatus{State: "closed"},
            wantOutcome: PROutcomeClosed,
        },
        {
            name: "pending checks",
            prStatus: PRStatus{State: "open"},
            checks: []CheckStatus{
                {Name: "ci/test", Status: "pending", Conclusion: ""},
            },
            wantOutcome:  PROutcomePending,
            wantAwaiting: false,
        },
        {
            name: "changes requested",
            prStatus: PRStatus{State: "open"},
            checks: []CheckStatus{
                {Name: "review/code", Status: "completed", Conclusion: "failure"},
            },
            wantOutcome:  PROutcomeChangesRequested,
            wantAwaiting: false,
        },
        {
            name: "approved",
            prStatus: PRStatus{State: "open"},
            checks: []CheckStatus{
                {Name: "review/code", Status: "completed", Conclusion: "success"},
            },
            wantOutcome:  PROutcomeApproved,
            wantAwaiting: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            store := &fakePollerStateStore{
                states: []*PRState{{SpecName: "spec" + tt.name, PRRef: PRRef{Number: 1}}},
            }
            client := &fakePollerPRClient{
                getPRFn: func(context.Context, PRRef) (PRStatus, error) {
                    return tt.prStatus, nil
                },
                listChecksFn: func(context.Context, PRRef) ([]CheckStatus, error) {
                    return tt.checks, nil
                },
            }
            poller := NewPoller(client, store)
            if err := poller.Poll(context.Background()); err != nil {
                t.Fatalf("Poll() error = %v", err)
            }
            state := store.states[0]
            if state.Outcome != tt.wantOutcome {
                t.Fatalf("outcome = %q, want %q", state.Outcome, tt.wantOutcome)
            }
            if state.AwaitingApproval != tt.wantAwaiting {
                t.Fatalf("awaiting approval = %v, want %v", state.AwaitingApproval, tt.wantAwaiting)
            }
            if len(tt.checks) != 0 && len(state.LastChecks) != len(tt.checks) {
                t.Fatalf("shared checks count = %d, want %d", len(state.LastChecks), len(tt.checks))
            }
        })
    }
}

type fakePollerStateStore struct {
    states []*PRState
}

func (f *fakePollerStateStore) List(context.Context) ([]*PRState, error) {
    return f.states, nil
}

func (f *fakePollerStateStore) Save(_ context.Context, state *PRState) error {
    for i, s := range f.states {
        if s.PRRef == state.PRRef {
            f.states[i] = state
            return nil
        }
    }
    f.states = append(f.states, state)
    return nil
}

type fakePollerPRClient struct {
    getPRFn     func(context.Context, PRRef) (PRStatus, error)
    listChecksFn func(context.Context, PRRef) ([]CheckStatus, error)
}

func (f *fakePollerPRClient) CreatePR(context.Context, string, string, string, string) (PRRef, error) {
    return PRRef{}, nil
}
func (f *fakePollerPRClient) GetPR(ctx context.Context, ref PRRef) (PRStatus, error) {
    if f.getPRFn != nil {
        return f.getPRFn(ctx, ref)
    }
    return PRStatus{}, nil
}
func (f *fakePollerPRClient) ListChecks(ctx context.Context, ref PRRef) ([]CheckStatus, error) {
    if f.listChecksFn != nil {
        return f.listChecksFn(ctx, ref)
    }
    return nil, nil
}
func (f *fakePollerPRClient) PostReview(context.Context, PRRef, ReviewPayload) error { return nil }
func (f *fakePollerPRClient) PostComment(context.Context, PRRef, string) error { return nil }
func (f *fakePollerPRClient) RequestReviewers(context.Context, PRRef, []string) error { return nil }
func (f *fakePollerPRClient) MergePR(context.Context, PRRef, string) error { return nil }
