package retro

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// AcceptedProposals contains the proposals accepted by the user
type AcceptedProposals struct {
	Consolidations []ConsolidationProposal
	Promotions     []PromotionProposal
	Archives       []ArchiveProposal
	RuleChanges    []RuleChangeProposal
}

// ReviewProposals walks through proposals interactively and collects accepted ones
func ReviewProposals(proposals *Proposals, reader io.Reader, writer io.Writer) (*AcceptedProposals, error) {
	if proposals == nil {
		return nil, fmt.Errorf("proposals is nil")
	}

	bufReader := bufio.NewReader(reader)
	accepted := &AcceptedProposals{}

	// Review consolidations
	if len(proposals.Consolidations) > 0 {
		fmt.Fprintf(writer, "\n=== CONSOLIDATIONS (%d) ===\n\n", len(proposals.Consolidations))
		for i, c := range proposals.Consolidations {
			fmt.Fprintf(writer, "[%d/%d] Consolidate learnings:\n", i+1, len(proposals.Consolidations))
			fmt.Fprintf(writer, "  Hashes: %s\n", strings.Join(c.LearningHashes, ", "))
			fmt.Fprintf(writer, "  Into: %s\n", c.ConsolidatedText)
			fmt.Fprintf(writer, "  Rationale: %s\n", c.Rationale)

			if accept, err := askYesNo(bufReader, writer, "Accept this consolidation?"); err != nil {
				return nil, err
			} else if accept {
				accepted.Consolidations = append(accepted.Consolidations, c)
			}
			fmt.Fprintln(writer)
		}
	}

	// Review promotions
	if len(proposals.Promotions) > 0 {
		fmt.Fprintf(writer, "\n=== PROMOTIONS (%d) ===\n\n", len(proposals.Promotions))
		for i, p := range proposals.Promotions {
			fmt.Fprintf(writer, "[%d/%d] Promote learning to rule:\n", i+1, len(proposals.Promotions))
			fmt.Fprintf(writer, "  Hash: %s\n", p.LearningHash)
			fmt.Fprintf(writer, "  Rule: %s\n", p.ProposedRule)
			fmt.Fprintf(writer, "  Section: %s\n", p.Section)
			fmt.Fprintf(writer, "  Rationale: %s\n", p.Rationale)

			if accept, err := askYesNo(bufReader, writer, "Accept this promotion?"); err != nil {
				return nil, err
			} else if accept {
				accepted.Promotions = append(accepted.Promotions, p)
			}
			fmt.Fprintln(writer)
		}
	}

	// Review archives
	if len(proposals.Archives) > 0 {
		fmt.Fprintf(writer, "\n=== ARCHIVES (%d) ===\n\n", len(proposals.Archives))
		for i, a := range proposals.Archives {
			fmt.Fprintf(writer, "[%d/%d] Archive learning:\n", i+1, len(proposals.Archives))
			fmt.Fprintf(writer, "  Hash: %s\n", a.LearningHash)
			fmt.Fprintf(writer, "  Rationale: %s\n", a.Rationale)

			if accept, err := askYesNo(bufReader, writer, "Accept this archive?"); err != nil {
				return nil, err
			} else if accept {
				accepted.Archives = append(accepted.Archives, a)
			}
			fmt.Fprintln(writer)
		}
	}

	// Review rule changes
	if len(proposals.RuleChanges) > 0 {
		fmt.Fprintf(writer, "\n=== RULE CHANGES (%d) ===\n\n", len(proposals.RuleChanges))
		for i, rc := range proposals.RuleChanges {
			fmt.Fprintf(writer, "[%d/%d] Change rule:\n", i+1, len(proposals.RuleChanges))
			fmt.Fprintf(writer, "  Current: %s\n", rc.CurrentRule)
			fmt.Fprintf(writer, "  Proposed: %s\n", rc.ProposedRule)
			fmt.Fprintf(writer, "  Rationale: %s\n", rc.Rationale)

			if accept, err := askYesNo(bufReader, writer, "Accept this change?"); err != nil {
				return nil, err
			} else if accept {
				accepted.RuleChanges = append(accepted.RuleChanges, rc)
			}
			fmt.Fprintln(writer)
		}
	}

	return accepted, nil
}

// askYesNo prompts the user for a yes/no answer
func askYesNo(reader *bufio.Reader, writer io.Writer, prompt string) (bool, error) {
	fmt.Fprintf(writer, "%s [y/n]: ", prompt)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("reading input: %w", err)
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes", nil
}
