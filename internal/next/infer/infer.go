package infer

import (
	"context"

	"github.com/danabrams/gromit/internal/next/fact"
)

type Inferrer interface {
	Infer(ctx context.Context, observed []fact.Fact) ([]fact.Fact, error)
}

type StubInferrer struct{}

func NewStubInferrer() *StubInferrer {
	return &StubInferrer{}
}

func (s *StubInferrer) Infer(ctx context.Context, observed []fact.Fact) ([]fact.Fact, error) {
	return []fact.Fact{}, nil
}
