# Plan (Cycle 2)

## t-014

Fix the 'spec picker — no eligible specs' contract in scenario-contracts.yaml: change the file_contains pattern from a YAML block scalar (|) to a plain quoted string 'no specs available to run' (without trailing newline), and change the test file target from 'cmd/gromit-next/exec_test.go' to 'cmd/gromit-next/exec_scenario_spec_picker_no_eligible_test.go' where the test actually lives. The YAML block scalar adds a literal newline character to the pattern which does not match the Go source (where \n is two literal characters in a string literal).

