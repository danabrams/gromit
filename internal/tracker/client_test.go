package tracker

import (
	"context"
	"testing"
)

type stubClient struct{}

func (stubClient) Ready(ctx context.Context) (*Item, error) {
	return nil, nil
}

func (stubClient) List(ctx context.Context, query Query) ([]Item, error) {
	return nil, nil
}

func (stubClient) Show(ctx context.Context, id string) (*Item, error) {
	return nil, nil
}

func (stubClient) Create(ctx context.Context, req CreateRequest) (*Item, error) {
	return nil, nil
}

func (stubClient) Close(ctx context.Context, id string) error {
	return nil
}

func (stubClient) Sync(ctx context.Context) error {
	return nil
}

func (stubClient) AddComment(ctx context.Context, id, comment string) error {
	return nil
}

func (stubClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return false, nil
}

func TestClientInterface(t *testing.T) {
	var _ Client = stubClient{}
}
