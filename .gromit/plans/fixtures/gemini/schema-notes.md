# Gemini Schema Notes

## Token and Cost Observations

- Command evidence is logged in `commands.log` with `# token-cost` tags.
- `gemini` is not installed in this environment, so runtime JSON fields could not be directly observed.
- Parser planning note: probe for `input_tokens`, `output_tokens`, and `cost` fields when Gemini JSON output is available.

## Model Observations

- Valid-model attempt artifact: `models/valid-model.stderr.txt` (`command not found` in this environment).
- Invalid-model attempt artifact: `models/invalid-model.stderr.txt` (`command not found` in this environment).
- Matching stdout captures are retained for completeness under `models/valid-model.stdout.txt` and `models/invalid-model.stdout.txt`.
