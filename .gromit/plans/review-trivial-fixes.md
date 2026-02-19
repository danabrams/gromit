# Review Trivial Fixes Plan

> From thorough code review on 2026-02-19. These are minor issues fixable in a single pass
> without dedicated beads. Each is a 1-5 line change with no behavioral impact.

## Fixes

### 1. Remove dead compile-time assertions in callbacks.go

**File:** `internal/runner/callbacks.go:525-526`

```go
// DELETE these two lines — they assert nil assignable to interface, which is always true
var _ provider.Provider = nil
var _ execution.Provider = nil
```

The idiomatic pattern `var _ T = (*ConcreteType)(nil)` is already correctly used in `interfaces.go:15-20`.

---

### 2. Remove dead `_ = &Runner{reviewer: nil}` in runner.go

**File:** `internal/runner/runner.go:112`

```go
// DELETE this line — exists only to satisfy AST inspection tests; restructure test instead
_ = &Runner{reviewer: nil}
```

---

### 3. Fix `FailedCriteria` returning nil instead of empty slice

**File:** `internal/specgate/verdict.go:31-39`

```go
// CHANGE: return []CriterionResult{} instead of nil for consistency with normalizeNilFields pattern
func (v GateVerdict) FailedCriteria() []CriterionResult {
    var failed []CriterionResult
    for _, cr := range v.Results {
        if !cr.Passed {
            failed = append(failed, cr)
        }
    }
    if failed == nil {
        failed = []CriterionResult{}  // ADD this normalization
    }
    return failed
}
```

---

### 4. Fix double-close of temp file in globalstats.go

**File:** `internal/logger/globalstats.go:104-121`

The deferred `tempFile.Close()` at line 105 runs after the explicit `tempFile.Close()` at line 119. Fix by removing the defer close and relying on the explicit close + error path close:

```go
// CHANGE: Remove defer tempFile.Close() at line 105
// Keep the explicit close at line 119 and add close in error paths
```

---

### 5. Fix `validationGateCancel` not using defer in process_methodology.go

**File:** `internal/runner/process_methodology.go:137-163`

```go
// CHANGE: Use defer immediately after creation
if atddActive || tddActive {
    validationGateCtx, validationGateCancel, valMeta = newPhaseContext(...)
    defer validationGateCancel()  // ADD defer here
}
// REMOVE the two manual validationGateCancel() calls below
```

---

### 6. Compile regexes once in lifecycle.go mandatoryCommandPattern

**File:** `internal/runner/lifecycle.go:546-558`

```go
// CHANGE: Move regexp.MustCompile to package-level vars
var (
    goTestPattern = regexp.MustCompile(`^go\s+test\b`)
    goVetPattern  = regexp.MustCompile(`^go\s+vet\b`)
    // ... etc
)

func mandatoryCommandPattern(requiredPrefix string) *regexp.Regexp {
    switch strings.TrimSpace(requiredPrefix) {
    case "go test":
        return goTestPattern
    // ...
    }
}
```

---

### 7. Remove trivial `scopeValidationCommands` wrapper in process.go

**File:** `internal/runner/process.go:229-231`

```go
// DELETE this function — pure delegation, replace all callers with config.ScopeGoTestCommands directly
func scopeValidationCommands(commands []string, touchedPackages []string) []string {
    return config.ScopeGoTestCommands(commands, touchedPackages)
}
```

---

### 8. Fix silently swallowed error from backlog.NewFile in refine.go

**File:** `cmd/gromit/refine.go:151`

```go
// CHANGE: Handle the error
bf, err := backlog.NewFile(gromitDir)
if err != nil {
    return fmt.Errorf("loading backlog: %w", err)
}
```

---

### 9. Normalize ProviderDef.ModelCosts in NormalizeNilFields

**File:** `internal/config/config.go:475-484`

```go
// ADD inside the Providers loop in NormalizeNilFields:
if p.ModelCosts == nil {
    p.ModelCosts = map[string]config.ModelCostDef{}
}
```

---

### 10. Add nil-check for r.renderer in handleInvokeError

**File:** `internal/runner/callbacks.go:149`

```go
// CHANGE: Add nil guard matching other renderer accesses in this file
case "bead":
    bc.Result.TimeoutType = "bead"
    if r.renderer != nil {
        escalation.ExtractTimeoutLearning(bc, r.renderer.GetLearningsFile())
    }
```

---

## Execution Order

1. Fixes 1, 2, 4, 5, 6, 7 (runner package) — run `go test ./internal/runner/... -count=1`
2. Fix 3 (specgate) — run `go test ./internal/specgate/... -count=1`
3. Fix 8 (refine) — run `go test ./cmd/gromit/... -count=1`
4. Fix 9 (config) — run `go test ./internal/config/... -count=1`
5. Fix 10 (callbacks) — run `go test ./internal/runner/... -count=1`
6. Final: `go build ./...` and `go vet ./...`
