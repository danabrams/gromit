# Plan (Cycle 3)

## t-017

Fix scenario-contracts.yaml: update all exec_test.go references to the correct scenario test file paths. The patterns exist in exec_scenario_spec_picker_test.go, exec_scenario_spec_picker_no_eligible_test.go, exec_scenario_resume_picker_test.go, exec_scenario_resume_picker_blocked_running_test.go, and exec_scenario_resume_explicit_id_test.go respectively. Addresses: all 'file_contains failed: pattern X not found in exec_test.go' contract failures.

## t-018

Implement pickSpec function in exec.go per the spec: discover specs, derive status, filter to ready/ready_for_review, display numbered list with worktree/branch info for ready_for_review specs, read selection from stdin. Must include the exact message 'no specs available to run\n' when no eligible specs exist. Also implement pickRun function in exec.go: list runs, filter to resumable statuses (StatusRunning, StatusNeedsHuman, StatusBlocked, StatusReadyForReview), sort by StartedAt descending, display with human-readable labels and timestamp format '2006-01-02 15:04:05', read selection. Addresses: 'no specs available to run' not found in exec.go, and unit test compilation failures for scenario tests that call pickSpec/pickRun.

