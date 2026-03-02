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

func (stubClient) Search(ctx context.Context, query Query) ([]Item, error) {
	return nil, nil
}

func (stubClient) Create(ctx context.Context, req CreateRequest) (*Item, error) {
	return nil, nil
}

func (stubClient) CreateWithParent(ctx context.Context, req CreateRequest, parentID string) (*Item, error) {
	return nil, nil
}

func (stubClient) Update(ctx context.Context, req UpdateRequest) (*Item, error) {
	return nil, nil
}

func (stubClient) ListWithLabel(ctx context.Context, label string) ([]Item, error) {
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

func TestItemReaderInterface(t *testing.T) {
	var _ ItemReader = stubClient{}
}

func TestItemWriterInterface(t *testing.T) {
	var _ ItemWriter = stubClient{}
}

func TestItemQueryInterface(t *testing.T) {
	var _ ItemQuery = stubClient{}
}

func TestItemReaderIncludesReadyWithLabel(t *testing.T) {
	type readerWithLabel interface {
		ItemReader
		ReadyWithLabel(ctx context.Context, label string) (*Item, error)
	}

	var _ readerWithLabel = stubClient{}
}
