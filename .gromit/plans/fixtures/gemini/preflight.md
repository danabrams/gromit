# Gemini Preflight

## Checklist

- [ ] Confirm CLI availability: `gemini --version`
- [ ] Confirm authenticated call path with a minimal prompt.

## Observed Results

- Pending first execution.

## Capture Harness

Use this pattern for every command capture:

```bash
cmd='gemini --version'
$cmd
exit_code=$?
printf 'timestamp=%s command="%s" exit_code=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$cmd" "$exit_code" >> .gromit/plans/fixtures/gemini/commands.log
```
