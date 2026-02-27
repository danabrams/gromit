package pipeline

import (
	"context"
	"errors"
	"testing"
)

func TestPipelineAddUnknownTypeHandoff(t *testing.T) {
	ctx := context.Background()
	client := &hookBacklogClient{
		addFunc: func(*Idea) error {
			t.Fatal("Add should not be called when type is unknown")
			return nil
		},
	}
	p := &Pipeline{deps: &Deps{BacklogClient: client}}

	result, err := p.Add(ctx, AddInput{Text: "Think about the architecture"})
	if !errors.Is(err, ErrUnknownIdeaType) {
		t.Fatalf("Add() error = %v, want ErrUnknownIdeaType", err)
	}
	if result != nil {
		t.Fatalf("Add() result = %#v, want nil", result)
	}
}

type hookBacklogClient struct {
	addFunc func(*Idea) error
}

func (h *hookBacklogClient) List() ([]*Idea, error) {
	return nil, nil
}

func (h *hookBacklogClient) Get(id string) (*Idea, error) {
	return nil, nil
}

func (h *hookBacklogClient) Add(item *Idea) error {
	if h.addFunc != nil {
		return h.addFunc(item)
	}
	return nil
}

func (h *hookBacklogClient) Update(id string, fn func(*Idea)) error {
	return nil
}
