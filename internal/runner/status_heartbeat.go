package runner

import (
	"sync"
	"time"
)

type statusHeartbeatParams struct {
	iteration         int
	beadID            string
	beadTitle         string
	model             string
	maxIterations     int
	timeBudgetMinutes int
	onWriteSuccess    func()
}

type statusHeartbeat struct {
	stopOnce sync.Once
	stopCh   chan struct{}
}

func (h *statusHeartbeat) Stop() {
	if h == nil {
		return
	}
	h.stopOnce.Do(func() { close(h.stopCh) })
}

func (r *Runner) startStatusHeartbeat(statusWriter *StatusWriter, p statusHeartbeatParams) *statusHeartbeat {
	h := &statusHeartbeat{stopCh: make(chan struct{})}
	if statusWriter == nil {
		return h
	}

	if err := statusWriter.Write(p.iteration, p.beadID, p.beadTitle, p.model, true, p.maxIterations, p.timeBudgetMinutes); err != nil {
		r.log("Warning: failed to write status.json: %v", err)
	} else if p.onWriteSuccess != nil {
		p.onWriteSuccess()
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := statusWriter.Write(p.iteration, p.beadID, p.beadTitle, p.model, true, p.maxIterations, p.timeBudgetMinutes); err != nil {
					r.log("Warning: failed to write status.json: %v", err)
				}
			case <-h.stopCh:
				return
			}
		}
	}()

	return h
}
