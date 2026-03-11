package specloop

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

func TestBudget_CyclesExhausted(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxSpecCycles: 2})
	b.IncrementCycle()
	b.IncrementCycle()
	if !b.CyclesExhausted() {
		t.Fatal("should be exhausted after 2 cycles with max=2")
	}
}

func TestBudget_HardBudgetExceeded_Cost(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 10.0, MaxSpecCycles: 99})
	b.AddCost(11.0)
	if !b.HardBudgetExceeded() {
		t.Fatal("should be exceeded when cost > max")
	}
}

func TestBudget_NotExceeded(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxSpecCycles: 5, MaxRunCostUSD: 50.0, MaxRunDurationSeconds: 3600})
	b.IncrementCycle()
	b.AddCost(5.0)
	if b.HardBudgetExceeded() {
		t.Fatal("should not be exceeded yet")
	}
}

func TestBudget_CyclesVsHardBudget_Distinct(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxSpecCycles: 2, MaxRunCostUSD: 50.0})
	b.IncrementCycle()
	b.IncrementCycle()
	if !b.CyclesExhausted() {
		t.Fatal("cycles should be exhausted")
	}
	if b.HardBudgetExceeded() {
		t.Fatal("hard budget should NOT be exceeded")
	}
}

func TestBudget_Reason(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})
	b.AddCost(1.5)
	reason := b.Reason()
	if reason == "" {
		t.Fatal("reason should be non-empty when budget exceeded")
	}
}

func TestBudget_Reason_Cycles(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99})
	b.IncrementCycle()
	reason := b.Reason()
	if reason == "" {
		t.Fatal("reason should be non-empty when cycles exhausted")
	}
}

func TestBudget_HardBudgetExceeded_TimeExceeded(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	b := NewBudgetWithClock(execpolicy.Budgets{MaxRunDurationSeconds: 60, MaxSpecCycles: 99}, clock)
	if b.HardBudgetExceeded() {
		t.Fatal("should not be exceeded at start")
	}
	clock.now = clock.now.Add(61 * time.Second)
	if !b.HardBudgetExceeded() {
		t.Fatal("should be exceeded after 61s with 60s limit")
	}
}

func TestBudget_Exceeded_CombinesCyclesAndHard(t *testing.T) {
	b := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99})
	if b.Exceeded() {
		t.Fatal("should not be exceeded initially")
	}
	b.IncrementCycle()
	if !b.Exceeded() {
		t.Fatal("should be exceeded after cycles exhausted")
	}
}
