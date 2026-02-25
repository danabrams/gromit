package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/experiment"
)

func TestRunnerHasExperimentManager(t *testing.T) {
	// Test that Runner has an experimentMgr field of type *experiment.Manager
	r := &Runner{
		cfg: nil,
		experimentMgr: experiment.NewManager([]*experiment.Experiment{}, ""),
	}
	if r.experimentMgr == nil {
		t.Fatal("experimentMgr should not be nil")
	}
}
