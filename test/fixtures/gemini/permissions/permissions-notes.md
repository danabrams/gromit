# provenance: Gemini CLI permission behavior verified with live gemini binary invocation on 2026-02-25.
# refresh: rerun permission tests against live Gemini binary and update approval mode evidence.
# Gemini Permissions Notes

## Scope

This fixture records Gemini CLI permission and approval mode behavior tested in non-interactive (headless) mode via direct Gemini invocations.

## Commands

Tested Gemini approval modes in headless mode:

1. `npm start --prefix /home/dabrams/gemini-cli -- --approval-mode yolo -p "What is 2+2?"`
2. `npm start --prefix /home/dabrams/gemini-cli -- --approval-mode default -p "What is 2+2?"`
3. `npm start --prefix /home/dabrams/gemini-cli -- --approval-mode plan -p "Write hello to /tmp/test.txt"`
4. `npm start --prefix /home/dabrams/gemini-cli -- --approval-mode auto_edit -p "What is the approval mode?"`

## Raw Evidence

### Approval Modes Supported

Gemini CLI supports the following approval modes (via `--approval-mode` flag):
- `default`: Prompts for user approval on tool/action execution (interactive mode only)
- `auto_edit`: Automatically approves edit-related tools/actions
- `yolo`: Automatically approves all tools/actions without prompting
- `plan`: Read-only mode - generates plans but cannot execute actions

### Headless Permission Behavior

When running in headless mode (`-p` flag for non-interactive):
- `--approval-mode yolo`: All operations are auto-approved; works in headless mode
- `--approval-mode default`: Falls back to auto-approval in headless mode (no interactive prompt)
- `--approval-mode plan`: Generates plans/analysis only; read-only mode active
- `--approval-mode auto_edit`: Auto-approves edit tools only; other operations may require approval

### Deprecated Flag

- `--allowed-tools`: Deprecated; policy engine should be used instead (via `--policy`)

## Observations

- Gemini CLI respects the `--approval-mode` flag for controlling permission/approval behavior
- In headless mode, interactive approval prompts are not available; approval modes determine auto-approval defaults
- The `yolo` mode provides maximum automation for CI/non-interactive environments
- The `plan` mode provides a read-only way to test/preview without executing actions
- Tool permission control has moved from `--allowed-tools` to the Policy Engine (`--policy` flag)
- Permissions are enforced at the Gemini CLI level, not the OS level
