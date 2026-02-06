# Learnings

Accumulated operational knowledge from Ralph iterations.
This file is automatically updated. Review periodically with `ralph retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

- **Config fields need defaults and tests**: Adding config struct fields without wiring defaults in `setDefaults()` causes repeated failures. Always add the default and a test that the zero-value YAML case works. (Observed: `ralph-runner-cmw` failed 66.7% until decomposed properly.)
- **Multi-file refactors must be split**: Beads that extract interfaces or refactor across 3+ files fail ~50% of the time. Decompose into one-interface-at-a-time or one-file-at-a-time beads. (Observed: `ralph-runner-thd` needed `complexity:high` label and multiple attempts.)
- **Beads requiring nonexistent CLI commands will always fail**: If a bead assumes a `bd` subcommand or external tool feature that doesn't exist, it will fail 100% of the time regardless of model. Verify the command exists before creating the bead. (Observed: `ralph-runner-jje` failed 6/6 attempts.)

---

## Provisional

*Seen once - may be specific to one task.*

- **Retro analysis should filter closed beads**: The retrospective flagged three "stuck" beads that were all already closed, generating noise. The stuck-bead detection should exclude beads with `status: closed`. (Tracked: `ralph-runner-ehn`)
