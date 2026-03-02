package main

import (
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

func newPipeline(cfg *config.Config, gromitDir string) (*pipeline.Pipeline, error) {
	deps, err := NewPipelineDeps(cfg, gromitDir)
	if err != nil {
		return nil, err
	}
	paths := &pipeline.Paths{GromitDir: gromitDir}
	return pipeline.New(deps, paths), nil
}
