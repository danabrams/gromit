# Plan (Cycle 10)

## t-043

Delete internal/next/specloop/specloop.go.bak — backup file must not be in version control (addresses review finding on specloop.go.bak:1)

## t-044

Fix re-run recovery path in write_scenario_tests.go: when an already-written test file exists but does not compile, delete the stale file and remove its manifest entry before falling through to the retry loop (addresses review finding on write_scenario_tests.go:128)

