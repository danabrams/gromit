// Package review implements Stage 4 of the Gromit pipeline: LLM code review.
// It is optional — only runs when configured. Invokes Claude to review changes
// and creates new beads from review findings.
package review
