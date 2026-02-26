package tracker

import (
	"context"
	"testing"
)

type stubClient struct{}

func (stubClient) List(ctx context.Context, query Query) ([]Item, error) {
	return nil, nil
}

func (stubClient) Get(ctx context.Context, id string) (*Item, error) {
	return nil, nil
}

func (stubClient) Create(ctx context.Context, req CreateRequest) (*Item, error) {
	return nil, nil
}

func (stubClient) UpdateStatus(ctx context.Context, id, status string) error {
	return nil
}

func TestClientInterface(t *testing.T) {
	var _ Client = stubClient{}
}
