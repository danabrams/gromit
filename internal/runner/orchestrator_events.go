package runner

import (
	"fmt"
	"os"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
)

func (o *Orchestrator) logInfo(format string, args ...any) {
	o.emitLog("info", format, args...)
}

func (o *Orchestrator) logWarning(format string, args ...any) {
	o.emitLog("warning", format, args...)
}

func (o *Orchestrator) emitLog(level string, format string, args ...any) {
	if o.emitter != nil && o.emitter.HasSubscribers() {
		emitterLogger := &events.EmitterLogger{Emitter: o.emitter}
		emitterLogger.Log(level, format, args...)
		return
	}

	output := o.cfg.Output
	if output == nil {
		output = os.Stderr
	}
	fmt.Fprintf(output, "[%s] %s\n", level, fmt.Sprintf(format, args...))
}

func (o *Orchestrator) emitBeadFailedEvent(b *bead.Bead, errMsg string) {
	if o.emitter == nil || b == nil {
		return
	}
	o.emitter.Emit(&events.BeadFailedEvent{
		BeadID:    b.ID,
		BeadTitle: b.Title,
		Error:     errMsg,
		Time:      time.Now(),
	})
}

func (o *Orchestrator) emitBeadStuckEvent(b *bead.Bead, reason string) {
	if o.emitter == nil || b == nil {
		return
	}
	o.emitter.Emit(&events.BeadStuckEvent{
		BeadID:    b.ID,
		BeadTitle: b.Title,
		Reason:    reason,
		Time:      time.Now(),
	})
}
