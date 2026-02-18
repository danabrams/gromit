package main

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var verifySpecCreateBeads bool

var verifySpecCmd = &cobra.Command{
	Use:   "verify-spec <spec>",
	Short: "Verify a spec's acceptance criteria",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	verifySpecCmd.Flags().BoolVar(&verifySpecCreateBeads, "create-beads", false, "Create fix beads for failing criteria")
	rootCmd.AddCommand(verifySpecCmd)
}

var acceptanceCriteriaNumbered = regexp.MustCompile(`^\\d+[.)]\\s+(.+)$`)

func extractAcceptanceCriteria(body string) ([]string, string) {
	lines := strings.Split(body, "\n")
	inSection := false
	var blockLines []string
	var criteria []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if inSection {
				break
			}
			if strings.EqualFold(trimmed, "## Acceptance Criteria") {
				inSection = true
			}
			continue
		}

		if !inSection {
			continue
		}

		blockLines = append(blockLines, line)

		switch {
		case strings.HasPrefix(trimmed, "- "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		case strings.HasPrefix(trimmed, "* "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "* ")))
		default:
			if matches := acceptanceCriteriaNumbered.FindStringSubmatch(trimmed); len(matches) == 2 {
				criteria = append(criteria, strings.TrimSpace(matches[1]))
			}
		}
	}

	block := strings.TrimSpace(strings.Join(blockLines, "\n"))
	if criteria == nil {
		criteria = []string{}
	}

	return criteria, block
}
