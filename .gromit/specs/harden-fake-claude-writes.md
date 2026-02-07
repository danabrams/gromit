---
id: harden-fake-claude-writes
source_ideas: [idea-1770459727449]
created: 2026-02-07
---

# Harden Fake Claude Script File Writes

## Specification

The fake claude script (`test/fakes/claude`) supports creating files during tests via `CLAUDE_WRITE_FILE` and `CLAUDE_WRITE_CONTENT` environment variables. Currently, the file path is used without validation in `mkdir -p` and a file write, and `echo` is used to write content which can misbehave with escape sequences.

The script must validate all file write inputs before performing filesystem operations:

1. **Require `TEST_DIR`**: When `CLAUDE_WRITE_FILE` is set, `TEST_DIR` must also be set. Without it, path containment cannot be verified.

2. **Require absolute path**: `CLAUDE_WRITE_FILE` must be an absolute path (starts with `/`). Relative paths would resolve against the working directory rather than `TEST_DIR`, bypassing containment.

3. **Path containment**: The resolved `CLAUDE_WRITE_FILE` must be within `TEST_DIR`. Use `realpath -m` (canonicalize without requiring existence) to resolve `..` components, then verify the result starts with the resolved `TEST_DIR` prefix. This prevents path traversal via sequences like `../../etc/passwd`.

4. **Fail loudly**: Any validation failure prints a descriptive error to stderr and exits with code 1, matching the script's existing error handling pattern (e.g., missing `CLAUDE_FIXTURE`).

5. **Use `printf` for content**: Replace `echo "$CLAUDE_WRITE_CONTENT"` with `printf '%s\n' "$CLAUDE_WRITE_CONTENT"` to ensure exact content fidelity regardless of backslashes, leading hyphens, or platform-specific `echo` behavior.

## Acceptance Criteria

- When `CLAUDE_WRITE_FILE` is set but `TEST_DIR` is not, the script exits 1 with an error message on stderr mentioning `TEST_DIR`.
- When `CLAUDE_WRITE_FILE` is a relative path, the script exits 1 with an error message on stderr mentioning "absolute path".
- When `CLAUDE_WRITE_FILE` resolves (via `realpath -m`) to a path outside `TEST_DIR`, the script exits 1 with an error message on stderr mentioning "outside TEST_DIR" or "path traversal".
- Content containing backslashes (e.g., `hello\nworld`) is written literally to the file, not interpreted as escape sequences.
- Existing happy-path behavior is preserved: valid absolute paths within `TEST_DIR` still create parent directories and write content with a trailing newline.

## Decisions

1. **Fail loudly, don't skip silently.** A path escaping the sandbox is a test setup bug. Failing immediately with a clear message makes it easy to diagnose, rather than producing confusing downstream failures. This matches the script's existing error conventions.

2. **Require `TEST_DIR` rather than falling back.** All test harnesses already set `TEST_DIR`. Allowing a fallback (like `/tmp`) would undermine the containment check. Better to surface the missing env var as an error.

3. **Use `realpath -m` for path resolution.** The target file and its parent directories may not exist yet (the script creates them). `realpath -m` resolves the canonical path without requiring existence. This is GNU coreutils and available on all Linux targets this project runs on.

4. **Use `printf '%s\n'` instead of `echo`.** `printf` behavior is specified by POSIX and does not vary between platforms. The `%s` format writes the argument literally. The `\n` in the format string provides the trailing newline that `echo` added, preserving compatibility with the existing contract test expectation.

## Research & Context

### Current State

The file write logic lives at `test/fakes/claude:20-29`:

```bash
if [[ -n "${CLAUDE_WRITE_FILE:-}" ]] && [[ -n "${CLAUDE_WRITE_CONTENT:-}" ]]; then
    parent_dir=$(dirname "$CLAUDE_WRITE_FILE")
    mkdir -p "$parent_dir"
    echo "$CLAUDE_WRITE_CONTENT" > "$CLAUDE_WRITE_FILE"
fi
```

### Test Coverage

- `test/contracts/fakes_integration_test.go:TestFakes_ClaudeWriteFile` — tests the happy path (valid path inside test dir, verifies content and parent dir creation). Line 264 has a comment acknowledging `echo` adds a newline.
- `test/e2e/refine_e2e_test.go` — uses `CLAUDE_WRITE_FILE` in three tests to simulate Claude creating spec files during refinement.

### Environment Setup

Both test harnesses set `TEST_DIR` to a temp directory:
- `test/e2e/e2e_test.go:187`
- `test/contracts/helpers_test.go:163`

The `bd` fake already uses `TEST_DIR` as its state directory (`test/fakes/bd:19`). The claude fake uses `${TEST_DIR:-/tmp}` for fail-once state files (lines 60, 100).
