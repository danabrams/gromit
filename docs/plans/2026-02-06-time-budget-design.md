# Time Budget Flag Design

Run as many iterations as possible within a time limit, so you can leave gromit running while you step away.

## Problem

The `-n` flag caps iterations by count, but often the real constraint is time: "I have 30 minutes before a meeting." Currently there's no way to express that. You either guess how many iterations fit, or run unlimited and Ctrl+C when you're back.

## Interface

Two new flags on `gromit run`:

```
gromit run -t 30                  # ~30 minutes
gromit run -H 1                   # ~1 hour
gromit run --time-budget 30       # same as -t 30
gromit run --time-budget-hours 1  # same as -H 1
gromit run -t 30 -H 1             # 90 minutes (additive)
gromit run -n 10 -t 30            # whichever limit hits first
```

- `-t` / `--time-budget`: integer, minutes
- `-H` / `--time-budget-hours`: integer, hours
- When both `-t` and `-H` are specified, they add together.
- When combined with `-n`, whichever limit hits first stops the loop.
- Runtime-only flags. No `gromit.yaml` schema changes.

## Loop Behavior

The deadline check goes at the top of the loop, right next to the existing iteration limit check -- before fetching the next bead. The current bead always finishes. We just don't start a new one after the deadline passes.

```go
// Existing check
if maxIterations > 0 && iteration >= maxIterations {
    r.log("Reached max iterations (%d), stopping", maxIterations)
    break
}

// New check
if !deadline.IsZero() && time.Now().After(deadline) {
    r.log("Time budget expired, stopping")
    break
}
```

This means the flag means "about X minutes," not "exactly X minutes." A run with `-t 30` might take 35 minutes if the last bead takes 5 minutes to finish. This matches the user's mental model -- "run while I'm at my meeting" -- and avoids wasting work by killing a bead mid-execution.

## Files Changed

1. **`cmd/gromit/main.go`** -- Two new vars (`timeBudgetMinutes`, `timeBudgetHours`), two flag registrations, compute `deadline` in `runLoop()`, pass to `r.Run()`.
2. **`internal/runner/runner.go`** -- Add `deadline time.Time` parameter to `Run()`, add one `if` check at the top of the loop.

No new packages. No new files. No config schema changes.
