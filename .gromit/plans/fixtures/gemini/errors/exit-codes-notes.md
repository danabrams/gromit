# provenance: deterministic shell-triggered exit-code evidence used while direct Gemini-specific non-zero semantics were unavailable.
# refresh: replace trigger attempts with live Gemini error captures when they become available; preserve section contract.
# Gemini Exit Code Notes

## Trigger Attempts

- exit code 0: `command="sh -c 'echo ok'"`
- exit code 1: `command="sh -c 'echo intentional trigger for exit code 1 >&2; exit 1'"`
- exit code 42: `command="sh -c 'echo intentional trigger for exit code 42 >&2; exit 42'"`
- exit code 53: `command="sh -c 'echo intentional trigger for exit code 53 >&2; exit 53'"`

## Observations

- These are shell-level trigger attempts used to prepare classifier-ready exit handling evidence while `gemini` is unavailable.
- `gemini` invocations in this environment currently fail before model execution with command-not-found behavior.
