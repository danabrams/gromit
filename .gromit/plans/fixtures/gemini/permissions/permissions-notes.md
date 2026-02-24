# provenance: deterministic permissions behavior notes from Gemini spike shell captures.
# refresh: rerun permission probes and replace evidence pointers with sanitized, stable paths/messages.
# Gemini Permissions Notes

## Scope

This fixture records command-backed permission observations from a non-interactive/headless shell session used for Gemini CLI spike work.

## Commands

1. `touch /root/gromit-permissions-check`
2. `d=$(mktemp -d); chmod 000 "$d"; ls "$d"`

## Raw Evidence

- `root-write.stderr.txt`: `touch: cannot touch '/root/gromit-permissions-check': Permission denied`
- `no-exec-dir.stderr.txt`: `ls: cannot open directory '/tmp/...': Permission denied`

## Observations

- Permission checks are enforced by the OS in headless mode (no interactive approval prompt is available here).
- Writes outside the user-accessible filesystem fail with `Permission denied`.
- Directory traversal/listing also fails when execute/read permissions are removed.
