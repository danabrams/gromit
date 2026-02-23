# Gemini Workdir Notes

## Scope

This fixture records command-backed working directory behavior observed in a non-interactive/headless shell session used for Gemini CLI spike work.

## Commands

1. `pwd`
2. `cd /tmp && pwd && ls /home/dabrams/gromit/.gromit/plans/fixtures/gemini/preflight.md`
3. `d=$(mktemp -d); cd "$d" && pwd && ls preflight.md`

## Raw Evidence

- `pwd.stdout.txt` shows the initial working directory as `/home/dabrams/gromit`.
- `tmp-absolute.stdout.txt` shows CWD switched to `/tmp` and absolute project paths remain accessible.
- `tmp-relative.stderr.txt` shows relative lookup failure from a different working directory: `No such file or directory`.

## Observations

- Commands resolve relative paths from the current working directory.
- Changing the working directory changes relative file resolution.
- Absolute paths work independently of the current working directory.
