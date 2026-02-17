package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func writeStatusFile(gromitDir string, status Status) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(gromitDir, "status.json"), data, 0644)
}
