---
id: branch-per-spec
source_ideas: []
created: 2026-02-26
accepted: true
---

# Branch per Spec ATDD Workflow

## Specification

Introduce a comprehensive Acceptance Test-Driven Development (ATDD) workflow that isolates feature development into spec-specific branches. Before implementation begins, the system designs acceptance tests and creates individual beads to write those tests. Implementation iterations occur on the spec branch, strictly guarded by these local spec acceptance tests. Upon successful implementation and code review, a global regression suite ensures no existing behaviors are broken before merging back to the main branch.

## Workflow State Tracking

State for a spec's workflow is persisted to allow safe interruptions and resumptions. A `stage` field will be added to the `.gromit/specs/<name>.md` frontmatter, tracking progress through the following phases:
1. `planning`
2. `acceptance-tests`
3. `implementation`
4. `local-gate`
5. `review`
6. `global-gate`
7. `done`

## Workflow

1. **Branch Creation:** `gromit run --spec <name>` checks the `stage` in the spec's frontmatter. If starting fresh, Gromit automatically creates and checks out a new branch for the spec (e.g., `gromit/spec-<name>`).
2. **Acceptance Test Generation:** During the `acceptance-tests` stage, `gromit decompose` (or the runner if automated) generates a plan for the acceptance tests that define the spec's behavioral contract. It creates individual beads (ideally one per test/scenario) to write and commit these tests to the branch.
3. **Implementation Loop:** In the `implementation` stage, the run loop executes the standard implementation beads on the spec branch. 
4. **Local Spec Gate:** At the end of the spec's run loop (`local-gate` stage), Gromit executes the acceptance tests written specifically for this spec. By convention, spec-specific tests should be identifiable (e.g., placed in `test/acceptance/<spec_name>/` or tagged appropriately) so Gromit can run them in isolation. If any tests fail, Gromit synthesizes fix beads and iterates until the spec gate passes.
5. **Review Phase:** In the `review` stage, the branch undergoes an automated code review via the existing `gromit review` agent. Fix beads are created for any review feedback. After fixes are applied, the local spec gate is executed again to ensure the fixes didn't break the contract.
6. **Global Regression Gate:** In the `global-gate` stage, the full suite of ATDD tests for the entire project is executed to ensure no regressions were introduced to other features.
7. **Resolution and Merge:** 
   - If global regressions occur, Gromit pauses, leaving the branch checked out for a human operator to decide if the new behavior intentionally supersedes the old tests or to manually resolve the conflict.
   - Once all gates pass (or regressions are human-approved), the spec branch is merged back into `main` and the stage is marked `done`.

## Acceptance Criteria

- `gromit run --spec <name>` automatically creates and switches to a `gromit/spec-<name>` branch.
- Pre-implementation beads are generated and executed to author the spec's acceptance tests before implementation beads run.
- A **Local Spec Gate** automatically runs only the acceptance tests associated with the current spec at the end of the implementation phase.
- Spec gate failures automatically generate fix beads on the spec branch, preventing the workflow from advancing to the review phase.
- An automated code review is triggered after the local spec gate passes, generating fix beads for any findings.
- A **Global Regression Gate** runs the full project test suite after the review phase is completed.
- Global regression failures halt the merge process, allowing a human operator to resolve the conflict or explicitly authorize the merge (superseding the broken tests).
- Upon passing the global regression gate (or upon human override), the spec branch is merged into `main`.
- The spec's progress is tracked via a `stage` field in its frontmatter, allowing the run loop to be interrupted and safely resumed.

## Decisions

1. **Test-First Bead Generation:** Creating explicit beads to author tests ensures the behavioral contract is completely established and committed to the branch before any implementation code is written.
2. **Layered Gating:** Separating the local spec gate from the global regression gate reduces feedback loop time during active development. The heavier, full-suite run is reserved for the final merge check.
3. **Human-in-the-Loop for Regressions:** While local spec failures are unambiguous and should be automatically fixed, global regressions might represent intentional behavioral changes. A human operator must arbitrate these conflicts before merging.
4. **Branch Isolation:** Using a dedicated branch per spec protects `main` from incomplete or failing implementations, keeping `main` clean and enabling safe, isolated execution.
5. **Automated Review:** The review phase leverages the existing autonomous review agent to catch style, logic, and security issues before the final global gate.
6. **Frontmatter State Tracking:** Storing the current stage in the spec document's frontmatter provides a visible, source-controlled way to track workflow progress.

## Related Specs

- `spec-level-atdd-execution`
- `atdd-methodology`
