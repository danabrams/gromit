package specloop

import (
	"fmt"
	"sync"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// Budget tracks resource consumption against configured limits.
type Budget struct {
	limits      execpolicy.Budgets
	cycles      int
	cost        float64
	startedAt   time.Time
	clock       Clock
	mu          sync.Mutex
	invocations []runstore.InvocationRecord
}

// Clock abstracts time for testing.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// NewBudget creates a Budget with the given limits.
func NewBudget(limits execpolicy.Budgets) *Budget {
	return &Budget{limits: limits, startedAt: time.Now(), clock: realClock{}}
}

// NewBudgetWithClock creates a Budget with a custom clock for testing.
func NewBudgetWithClock(limits execpolicy.Budgets, clock Clock) *Budget {
	return &Budget{limits: limits, startedAt: clock.Now(), clock: clock}
}

// IncrementCycle records one cycle consumed.
func (b *Budget) IncrementCycle() { b.cycles++ }

// AddCost records cost consumed.
func (b *Budget) AddCost(usd float64) { b.cost += usd }

// AddInvocation appends an invocation record to the budget's log.
func (b *Budget) AddInvocation(r runstore.InvocationRecord) {
	b.mu.Lock()
	b.invocations = append(b.invocations, r)
	b.mu.Unlock()
}

// GetInvocations returns a copy of all recorded invocations.
func (b *Budget) GetInvocations() []runstore.InvocationRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]runstore.InvocationRecord, len(b.invocations))
	copy(out, b.invocations)
	return out
}

// Cost returns the total cost accumulated so far.
func (b *Budget) Cost() float64 { return b.cost }

// CyclesExhausted returns true when all allowed cycles have been used.
func (b *Budget) CyclesExhausted() bool {
	return b.cycles >= b.limits.MaxSpecCycles
}

// HardBudgetExceeded returns true if cost or time limits are exceeded.
// Cycle exhaustion is not a hard budget — it is handled separately.
func (b *Budget) HardBudgetExceeded() bool {
	if b.limits.MaxRunCostUSD > 0 && b.cost >= b.limits.MaxRunCostUSD {
		return true
	}
	if b.limits.MaxRunDurationSeconds > 0 {
		elapsed := b.clock.Now().Sub(b.startedAt).Seconds()
		if elapsed >= float64(b.limits.MaxRunDurationSeconds) {
			return true
		}
	}
	return false
}

// Exceeded returns true if any budget limit has been reached.
func (b *Budget) Exceeded() bool {
	return b.CyclesExhausted() || b.HardBudgetExceeded()
}

// MaxCycles returns the configured maximum number of spec cycles.
func (b *Budget) MaxCycles() int { return b.limits.MaxSpecCycles }

// Reason returns a human-readable explanation of which budget was exceeded.
func (b *Budget) Reason() string {
	if b.limits.MaxRunCostUSD > 0 && b.cost >= b.limits.MaxRunCostUSD {
		return fmt.Sprintf("cost budget exceeded: $%.2f >= $%.2f", b.cost, b.limits.MaxRunCostUSD)
	}
	if b.limits.MaxRunDurationSeconds > 0 {
		elapsed := b.clock.Now().Sub(b.startedAt).Seconds()
		if elapsed >= float64(b.limits.MaxRunDurationSeconds) {
			return fmt.Sprintf("time budget exceeded: %.0fs >= %ds", elapsed, b.limits.MaxRunDurationSeconds)
		}
	}
	if b.CyclesExhausted() {
		return fmt.Sprintf("cycle budget exhausted: %d >= %d", b.cycles, b.limits.MaxSpecCycles)
	}
	return ""
}
