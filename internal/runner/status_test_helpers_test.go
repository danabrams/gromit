package runner

import (
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

func withFastStatusReaders(t testingT) {
	t.Helper()

	prevPipeline := readPipelineStatus
	prevModelStats := readModelStats

	readPipelineStatus = func(gromitDir, specsDir, plansDir string, startedAt *time.Time) (*pipeline.PipelineStatus, error) {
		return &pipeline.PipelineStatus{
			Recommendation: "No work in pipeline",
		}, nil
	}
	readModelStats = func(logsDir string) (map[string]logger.ModelStats, error) {
		return map[string]logger.ModelStats{}, nil
	}

	t.Cleanup(func() {
		readPipelineStatus = prevPipeline
		readModelStats = prevModelStats
	})
}

type testingT interface {
	Helper()
	Cleanup(func())
}
