# Plan (Cycle 10)

## t-042

Add `store *runstore.Store` field to execSpecRun, construct store in RunE before picker calls, pass it into the struct literal, and replace all uses of local `store` in run() with `e.store`. Addresses review finding: execSpecRun lacks a store field and run() redundantly creates a local store.

## t-043

Update all existing exec spec tests to pass the new `store` field on execSpecRun struct literals, ensuring all tests compile and pass after the struct change in t-042.

