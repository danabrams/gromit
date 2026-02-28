package runner

import (
	"errors"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner/display"
)

// StatusJSON represents the structured payload returned by gromit status --json.
type StatusJSON struct {
	Run              *Status                         `json:"run,omitempty"`
	Pipeline         *pipeline.PipelineStatus        `json:"pipeline,omitempty"`
	IntegrationQueue *display.IntegrationQueueStatus `json:"integration_queue,omitempty"`
	Errors           []string                        `json:"errors,omitempty"`
}

// BuildStatusJSON gathers all status sections into a structured object.
func BuildStatusJSON(gromitDir string, cfg *config.Config) (*StatusJSON, error) {
	if cfg != nil {
		cfg.SetDefaults()
	}

	status, err := ReadStatus(gromitDir)
	if err != nil {
		return nil, err
	}
	if status != nil {
		refreshScopedIterationTotal(status, gromitDir)
	}

	var startedAt *time.Time
	if status != nil && !status.StartedAt.IsZero() {
		startedAt = &status.StartedAt
	}

	pipelineStatus, err := pipeline.ReadStatus(gromitDir, cfg.Paths.Specs, cfg.Paths.Plans, startedAt)
	if err != nil {
		return nil, err
	}

	queueStatus, queueErr := ReadIntegrationQueue(gromitDir)
	queueSchemaInvalid := errors.Is(queueErr, integrationqueue.ErrSchemaInvalid)
	if queueErr != nil && !queueSchemaInvalid {
		return nil, fmt.Errorf("reading integration queue status: %w", queueErr)
	}

	result := &StatusJSON{
		Run:              status,
		Pipeline:         pipelineStatus,
		IntegrationQueue: queueStatus,
	}
	if queueSchemaInvalid {
		result.Errors = append(result.Errors, "queue_schema_invalid")
	}
	return result, nil
}
