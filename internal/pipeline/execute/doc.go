// Package execute implements Stage 2 of the Gromit pipeline: LLM code authoring.
// It selects methodology (TDD, refactor, standard), constructs the prompt,
// invokes Claude, and handles internal escalation (haiku→sonnet→opus).
package execute
