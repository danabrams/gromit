package stage

import (
	"context"

	"github.com/danabrams/gromit/internal/config"
)

// Decision represents the advice returned by a stage.
type Decision int

const (
	DecisionProceed Decision = iota
	DecisionSkip
	DecisionBlock
)

// Request carries the spec metadata that stages consume.
type Request struct {
	SpecID string
	Config *config.Config
}

// Result reports the outcome of running a stage.
type Result struct {
	Summary  string
	Decision Decision
}

// Stage defines the contract every execution stage must honor.
type Stage interface {
	Name() string
	Run(ctx context.Context, req *Request) (*Result, error)
}
