package trackertest

import (
	"context"

	"github.com/danabrams/gromit/internal/tracker"
)

// StubTrackerClient lets tests fake tracker.Client implementations.
type StubTrackerClient struct {
	ReadyFn            func(ctx context.Context) (*tracker.Item, error)
	ReadyWithLabelFn   func(ctx context.Context, label string) (*tracker.Item, error)
	ListFn             func(ctx context.Context, q tracker.Query) ([]tracker.Item, error)
	ListWithLabelFn    func(ctx context.Context, label string) ([]tracker.Item, error)
	ShowFn             func(ctx context.Context, id string) (*tracker.Item, error)
	SearchFn           func(ctx context.Context, q tracker.Query) ([]tracker.Item, error)
	CreateFn           func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error)
	CreateWithParentFn func(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error)
	UpdateFn           func(ctx context.Context, req tracker.UpdateRequest) (*tracker.Item, error)
	CloseFn            func(ctx context.Context, id string) error
	SyncFn             func(ctx context.Context) error
	AddCommentFn       func(ctx context.Context, id, comment string) error
	HasOpenChildrenFn  func(ctx context.Context, parentID string) (bool, error)
}

// NewStubTrackerClient returns a stub with no-op behaviors.
func NewStubTrackerClient() *StubTrackerClient {
	return &StubTrackerClient{}
}

var _ tracker.Client = (*StubTrackerClient)(nil)

func (s *StubTrackerClient) Ready(ctx context.Context) (*tracker.Item, error) {
	if s == nil || s.ReadyFn == nil {
		return nil, nil
	}
	return s.ReadyFn(ctx)
}

func (s *StubTrackerClient) ReadyWithLabel(ctx context.Context, label string) (*tracker.Item, error) {
	if s == nil || s.ReadyWithLabelFn == nil {
		return nil, nil
	}
	return s.ReadyWithLabelFn(ctx, label)
}

func (s *StubTrackerClient) List(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	if s == nil || s.ListFn == nil {
		return nil, nil
	}
	return s.ListFn(ctx, q)
}

func (s *StubTrackerClient) Show(ctx context.Context, id string) (*tracker.Item, error) {
	if s == nil || s.ShowFn == nil {
		return nil, nil
	}
	return s.ShowFn(ctx, id)
}

func (s *StubTrackerClient) Search(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	if s == nil || s.SearchFn == nil {
		return nil, nil
	}
	return s.SearchFn(ctx, q)
}

func (s *StubTrackerClient) Create(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
	if s == nil || s.CreateFn == nil {
		return nil, nil
	}
	return s.CreateFn(ctx, req)
}

func (s *StubTrackerClient) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
	if s == nil || s.CreateWithParentFn == nil {
		return nil, nil
	}
	return s.CreateWithParentFn(ctx, req, parentID)
}

func (s *StubTrackerClient) Update(ctx context.Context, req tracker.UpdateRequest) (*tracker.Item, error) {
	if s == nil || s.UpdateFn == nil {
		return nil, nil
	}
	return s.UpdateFn(ctx, req)
}

func (s *StubTrackerClient) ListWithLabel(ctx context.Context, label string) ([]tracker.Item, error) {
	if s == nil || s.ListWithLabelFn == nil {
		return nil, nil
	}
	return s.ListWithLabelFn(ctx, label)
}

func (s *StubTrackerClient) Close(ctx context.Context, id string) error {
	if s == nil || s.CloseFn == nil {
		return nil
	}
	return s.CloseFn(ctx, id)
}

func (s *StubTrackerClient) Sync(ctx context.Context) error {
	if s == nil || s.SyncFn == nil {
		return nil
	}
	return s.SyncFn(ctx)
}

func (s *StubTrackerClient) AddComment(ctx context.Context, id, comment string) error {
	if s == nil || s.AddCommentFn == nil {
		return nil
	}
	return s.AddCommentFn(ctx, id, comment)
}

func (s *StubTrackerClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	if s == nil || s.HasOpenChildrenFn == nil {
		return false, nil
	}
	return s.HasOpenChildrenFn(ctx, parentID)
}
