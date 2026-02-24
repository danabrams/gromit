# Curated Metrics Artifacts

This directory is for deterministic report artifacts that are safe to version in git.

Policy:
- Do not commit raw runtime captures here.
- Do not include timestamps in filenames.
- Normalize ordering/encoding so reruns produce stable diffs.
- Keep volatile per-run outputs under ignored paths (for example `.gromit/reports/runs/`).
