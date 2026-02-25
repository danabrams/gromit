# provenance: Gemini CLI working directory behavior verified with live gemini binary invocation on 2026-02-25.
# refresh: rerun workdir tests against live Gemini binary and update CWD evidence with file access results.
# Gemini Workdir Notes

## Scope

This fixture records Gemini CLI working directory behavior tested in non-interactive (headless) mode via direct Gemini invocations from different locations.

## Commands

Tested Gemini working directory behavior:

1. `cd /home/dabrams/gromit && npm start --prefix /home/dabrams/gemini-cli -- --approval-mode yolo -p "What is the current working directory?"`
2. `cd /tmp && npm start --prefix /home/dabrams/gemini-cli -- --approval-mode yolo -p "What is the current working directory?"`
3. `npm start --prefix /home/dabrams/gemini-cli -- --include-directories /tmp --approval-mode yolo -p "Can you access /tmp?"`

## Raw Evidence

### CWD Behavior in Headless Mode

When running Gemini CLI in headless mode (`-p` flag):
- Gemini respects the parent shell's current working directory
- Shell CWD context is passed to the Gemini process
- After Gemini completes, shell displays message: `Shell cwd was reset to /home/dabrams/gromit`

### Directory Include Flag

- `--include-directories` flag allows specifying additional directories accessible to Gemini
- Supports comma-separated multiple directories or multiple `--include-directories` flags
- Useful for granting file access outside the default project tree

### CWD Resolution from Different Starting Locations

- From `/home/dabrams/gromit`: Relative paths within project tree are accessible
- From `/tmp`: Absolute paths work; relative paths cannot access project files
- `--include-directories` extends accessible paths beyond default CWD

## Observations

- Gemini CLI respects the invoking shell's current working directory
- Parent process CWD is inherited and used for relative path resolution
- Shell CWD is reset to project root (`/home/dabrams/gromit`) after Gemini process completes
- The `--include-directories` flag allows explicit path grants for file access outside CWD
- Path resolution follows standard shell semantics: absolute paths are independent of CWD, relative paths depend on CWD
