package specmerge

import (
	"context"
	"time"
)

// CycleRecord captures the data emitted when a spec presentation is ready for review.
type CycleRecord struct {
	SpecID              string
	CycleEndPresentedAt time.Time
}

// CycleRecordEmitter captures cycle records for downstream persistence or validation.
type CycleRecordEmitter interface {
	CaptureCycleRecord(ctx context.Context, record CycleRecord) error
}

type noopCycleRecordEmitter struct{}

// CaptureCycleRecord is a no-op implementation that accepts any record without error.
func (noopCycleRecordEmitter) CaptureCycleRecord(context.Context, CycleRecord) error {
	return nil
}

// NoopCycleRecordEmitter returns a CycleRecordEmitter that does nothing.
func NoopCycleRecordEmitter() CycleRecordEmitter {
	return noopCycleRecordEmitter{}
}
