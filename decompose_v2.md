# Decompose Plan: %s

You are decomposing an implementation plan into bd beads following the gromit-decompose skill.

## Plan Content

%s

## Skill Instructions

%s

## Output

Output ONLY a JSON array of bead definitions. No markdown, no explanations, no wrapper.
Each bead must include: title, description, priority, acceptance_criteria, expected_outputs, covers_tasks, depends_on_index.

expected_outputs: list each individual deliverable, function, or independently testable item as a separate entry. These drive TDD RED-GREEN cycles — one cycle per entry. Do not summarize or group; enumerate fine-grained items.
covers_tasks: list the 1-based Task numbers from the plan that this bead covers. Every Task in the plan must be covered by at least one bead.
depends_on_index: array of 0-based indices of prerequisite beads in THIS output array. If bead at index 2 needs types or functions introduced by beads at indices 0 and 1, set "depends_on_index": [0, 1]. Plans with sequential tasks MUST produce dependency chains — only root beads with no prerequisites should have an empty array. Most beads should depend on at least one earlier bead.

The spec label will be added automatically: spec:%s
