package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/proposaltriage"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/spf13/cobra"
)

// buildProposalsCompleter creates an LLMCompleter for proposal grouping.
// Reads distiller_tier from project config, falling back to TierMedium if config is unavailable.
// Returns nil if completer creation fails (non-blocking); errors are logged.
func buildProposalsCompleter(storeDir string) reviewdistiller.LLMCompleter {
	const defaultClaudeBinary = "claude"

	// Load project ID and construct project directory
	projectID := loadProjectID(storeDir)
	tier := reviewdistiller.TierMedium
	if projectID != "" {
		projectDir := filepath.Join(storeDir, "projects", projectID)
		cfg, err := LoadProjectConfig(projectDir)
		if err != nil {
			log.Printf("warning: load project config: %v, using default tier", err)
		} else {
			tier = reviewdistiller.Tier(cfg.DistillerTier)
		}
	}

	defaultPolicy := execpolicy.DefaultPolicy()
	client, err := claude.NewClient(defaultClaudeBinary, nil, defaultPolicy.Budgets.MaxTaskDurationSeconds)
	if err != nil {
		log.Printf("warning: create claude client for proposal grouping: %v", err)
		return nil
	}

	prov := provider.NewClaudeProvider(client, provider.DefaultTierToModelMap)
	adapter := llmadapter.New(prov, llmadapter.Config{
		Phase: "review",
		Tier:  string(tier),
	})

	return NewInvokerAdapter(adapter)
}

var proposalsCmd = &cobra.Command{
	Use:   "proposals",
	Short: "Review and triage improvement proposals",
}

// newReviewProposalsListCmd creates the `review proposals list` command.
func newReviewProposalsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List improvement proposals for review",
		Long: `List improvement proposals from distillation across runs.
By default, shows only pending proposals. Use --all to include accepted and rejected proposals.
Filter by --type and --run as needed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			storeDir, _ := cmd.Flags().GetString("store-dir")
			typeStr, _ := cmd.Flags().GetString("type")
			runID, _ := cmd.Flags().GetString("run")
			showAll, _ := cmd.Flags().GetBool("all")

			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			// Load project ID from store
			projectID := loadProjectID(storeDir)
			if projectID == "" {
				return fmt.Errorf("could not determine project ID from store")
			}

			// Build filters
			var typeFilter *[]string
			if typeStr != "" {
				typeFilter = &[]string{typeStr}
			}

			var runFilter *[]string
			if runID != "" {
				runFilter = &[]string{runID}
			}

			// Discover proposals
			if showAll {
				allProposals, err := proposaltriage.DiscoverAll(storeDir, projectID, typeFilter, runFilter)
				if err != nil {
					return fmt.Errorf("discover proposals: %w", err)
				}
				return displayAllProposals(allProposals)
			}

			// For pending proposals, apply grouping pipeline
			pendingProposals, err := proposaltriage.DiscoverPending(storeDir, projectID, nil, runFilter)
			if err != nil {
				return fmt.Errorf("discover proposals: %w", err)
			}

			// Build completer for grouping (non-blocking if it fails)
			completer := buildProposalsCompleter(storeDir)

			// Run grouping pipeline
			groups, warnings := proposaltriage.GroupProposals(context.Background(), pendingProposals, completer)

			// Display warnings from LLM clustering failures
			for _, warning := range warnings {
				fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
			}

			// Apply type filtering after grouping (so mixed-type clusters can form)
			if typeFilter != nil && len(*typeFilter) > 0 {
				groups = proposaltriage.FilterGroupsByType(groups, *typeFilter)
			}

			return displayPendingProposals(groups)
		},
	}

	cmd.Flags().String("store-dir", "", "Override store directory (default: .gromit-next)")
	cmd.Flags().String("type", "", "Filter by proposal type (doctrine_rule, validation_gap, planner_heuristic, refinement_guidance)")
	cmd.Flags().String("run", "", "Filter by source run ID")
	cmd.Flags().Bool("all", false, "Show all proposals including accepted and rejected")

	return cmd
}

// newReviewProposalsShowCmd creates the `review proposals show <id>` command.
func newReviewProposalsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <proposal-id>",
		Short: "Show full details of a proposal",
		Long:  `Display complete details of a proposal including all fields, source run context, and any decision.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proposalID := args[0]
			storeDir, _ := cmd.Flags().GetString("store-dir")

			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			// Load project ID from store
			projectID := loadProjectID(storeDir)
			if projectID == "" {
				return fmt.Errorf("could not determine project ID from store")
			}

			// Discover all proposals to find the matching one
			allProposals, err := proposaltriage.DiscoverAll(storeDir, projectID, nil, nil)
			if err != nil {
				return fmt.Errorf("discover proposals: %w", err)
			}

			// Find the proposal matching the given ID
			var targetProposal *proposaltriage.AllProposal
			for i := range allProposals {
				if allProposals[i].Proposal != nil && allProposals[i].Proposal.ID == proposalID {
					targetProposal = &allProposals[i]
					break
				}
			}

			if targetProposal == nil {
				return fmt.Errorf("proposal with ID %q not found", proposalID)
			}

			return displayProposalDetail(targetProposal)
		},
	}

	cmd.Flags().String("store-dir", "", "Override store directory (default: .gromit-next)")

	return cmd
}

// newReviewProposalsAcceptCmd creates the `review proposals accept <proposal-id>` command.
func newReviewProposalsAcceptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept <proposal-id>",
		Short: "Accept an improvement proposal",
		Long: `Accept a proposal and materialize it into doctrine or playbook.
Optionally override fields with --title, --change, or --rationale flags.
Use --dismiss-group to also dismiss sibling proposals in the same group.
The decision is saved and the materialized entry ID is reported.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proposalID := args[0]
			storeDir, _ := cmd.Flags().GetString("store-dir")
			overrideTitle, _ := cmd.Flags().GetString("title")
			overrideChange, _ := cmd.Flags().GetString("change")
			overrideRationale, _ := cmd.Flags().GetString("rationale")
			scope, _ := cmd.Flags().GetString("scope")
			dismissGroup, _ := cmd.Flags().GetBool("dismiss-group")

			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			// Validate scope
			if scope != "local" && scope != "global" {
				return fmt.Errorf("invalid scope %q: must be 'local' or 'global'", scope)
			}

			// Load project ID from store
			projectID := loadProjectID(storeDir)
			if projectID == "" {
				return fmt.Errorf("could not determine project ID from store")
			}

			// Discover all proposals to find the matching one
			allProposals, err := proposaltriage.DiscoverAll(storeDir, projectID, nil, nil)
			if err != nil {
				return fmt.Errorf("discover proposals: %w", err)
			}

			// Find the proposal matching the given ID
			var targetProposal *proposaltriage.AllProposal
			for i := range allProposals {
				if allProposals[i].Proposal != nil && allProposals[i].Proposal.ID == proposalID {
					targetProposal = &allProposals[i]
					break
				}
			}

			if targetProposal == nil {
				return fmt.Errorf("proposal with ID %q not found", proposalID)
			}

			// Check if proposal is already decided
			if targetProposal.Decision != nil {
				return fmt.Errorf("proposal %q already has a decision: %s", proposalID, targetProposal.Decision.Action)
			}

			// Handle --dismiss-group: discover and group BEFORE promoting
			// (so the accepted proposal is still pending and can be found in its group)
			var acceptedGroup *proposaltriage.ProposalGroup
			if dismissGroup {
				// Discover pending proposals for grouping
				pendingProposals, err := proposaltriage.DiscoverPending(storeDir, projectID, nil, nil)
				if err != nil {
					return fmt.Errorf("discover pending proposals for grouping: %w", err)
				}

				// Build completer for grouping (non-blocking if it fails)
				completer := buildProposalsCompleter(storeDir)

				// Run full grouping pipeline (exact hash + LLM semantic clustering)
				groups, warnings := proposaltriage.GroupProposals(context.Background(), pendingProposals, completer)

				// Log warnings from LLM clustering failures
				for _, warning := range warnings {
					fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
				}

				// Find the accepted proposal's group
				for i := range groups {
					for _, pp := range groups[i].Proposals {
						if pp.Proposal != nil && pp.Proposal.ID == proposalID {
							acceptedGroup = &groups[i]
							break
						}
					}
					if acceptedGroup != nil {
						break
					}
				}
			}

			// Resolve store paths based on scope
			var doctrineDir, playbookDir string
			if scope == "global" {
				doctrineDir = filepath.Join(storeDir, "global", "doctrine")
				playbookDir = filepath.Join(storeDir, "global", "playbook")
			} else {
				projectDir := filepath.Join(storeDir, "projects", projectID)
				doctrineDir = filepath.Join(projectDir, "doctrine")
				playbookDir = filepath.Join(projectDir, "playbook")
			}

			// Create stores
			doctrineStore := doctrine.NewFSStore()
			doctrineStore.Dir = doctrineDir
			playbookStore := &playbook.Store{Dir: playbookDir}

			// Create pending proposal wrapper for Accept
			pp := &proposaltriage.PendingProposal{
				Proposal: targetProposal.Proposal,
				RunID:    targetProposal.RunID,
				SpecID:   targetProposal.SpecID,
			}
			// Get evidence directory before promoting
			runStore := runstore.NewStore(storeDir)
			runEvidenceDir := runStore.RunEvidenceDir(targetProposal.RunID)

			// Call Promote to create decision
			decision, err := proposaltriage.Promote(
				pp,
				overrideTitle,
				overrideChange,
				overrideRationale,
				doctrineStore,
				playbookStore,
				scope,
				runEvidenceDir,
			)
			if err != nil {
				return fmt.Errorf("accept proposal: %w", err)
			}

			// Save decision to run's evidence directory
			if err := proposaltriage.SaveDecisions(runEvidenceDir, []proposaltriage.Decision{*decision}); err != nil {
				return fmt.Errorf("save decision: %w", err)
			}

			// Determine target store based on proposal type
			targetStore := "playbook"
			if targetProposal.Proposal.Type == "doctrine_rule" {
				targetStore = "doctrine"
			}

			// Report results
			fmt.Printf("Proposal %q accepted\n", proposalID)
			fmt.Printf("Materialized ID: %s\n", decision.MaterializedID)
			fmt.Printf("Target store: %s\n", targetStore)
			if decision.DuplicateOf != "" {
				fmt.Printf("Note: Duplicate of existing entry %s (not materialized)\n", decision.DuplicateOf)
			}

			// Dismiss siblings if group was found.
			// NOTE: DismissSiblings is intentionally non-fatal. The accept+materialize operation
			// (the critical path) has already been persisted to disk. If DismissSiblings fails,
			// we log a warning and continue, allowing the accept to be reported as successful.
			// The operator can re-run dismiss-group on any failed siblings without affecting
			// the already-accepted proposal.
			if dismissGroup && acceptedGroup != nil {
				dismissedDecisions, err := proposaltriage.DismissSiblings(proposalID, *acceptedGroup, runStore)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: dismiss siblings failed: %v (re-run dismiss-group to retry)\n", err)
				} else {
					fmt.Printf("Dismissed %d sibling proposal(s) in the same group\n", len(dismissedDecisions))
				}
			}

			return nil
		},
	}

	cmd.Flags().String("store-dir", "", "Override store directory (default: .gromit-next)")
	cmd.Flags().String("title", "", "Override proposal title")
	cmd.Flags().String("change", "", "Override proposed change description")
	cmd.Flags().String("rationale", "", "Override rationale")
	cmd.Flags().String("scope", "local", "Scope for materialization: 'local' (default) or 'global'")
	cmd.Flags().Bool("dismiss-group", false, "Dismiss sibling proposals in the same group")

	return cmd
}

// newReviewProposalsRejectCmd creates the `review proposals reject <proposal-id>` command.
func newReviewProposalsRejectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject <proposal-id>",
		Short: "Reject an improvement proposal",
		Long: `Reject a proposal. If the proposal was previously accepted, the materialized entry
is marked as superseded. The decision is saved and the result is reported.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proposalID := args[0]
			storeDir, _ := cmd.Flags().GetString("store-dir")
			reason, _ := cmd.Flags().GetString("reason")

			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			// Reason is required
			if reason == "" {
				return fmt.Errorf("--reason flag is required")
			}

			// Load project ID from store
			projectID := loadProjectID(storeDir)
			if projectID == "" {
				return fmt.Errorf("could not determine project ID from store")
			}

			// Discover all proposals to find the matching one
			allProposals, err := proposaltriage.DiscoverAll(storeDir, projectID, nil, nil)
			if err != nil {
				return fmt.Errorf("discover proposals: %w", err)
			}

			// Find the proposal matching the given ID
			var targetProposal *proposaltriage.AllProposal
			for i := range allProposals {
				if allProposals[i].Proposal != nil && allProposals[i].Proposal.ID == proposalID {
					targetProposal = &allProposals[i]
					break
				}
			}

			if targetProposal == nil {
				return fmt.Errorf("proposal with ID %q not found", proposalID)
			}

			// Create rejection decision
			pp := &proposaltriage.PendingProposal{
				Proposal: targetProposal.Proposal,
				RunID:    targetProposal.RunID,
				SpecID:   targetProposal.SpecID,
			}

			rejectionDecision, err := proposaltriage.Reject(pp, reason)
			if err != nil {
				return fmt.Errorf("create rejection decision: %w", err)
			}

			// Resolve project cell paths for doctrine and playbook stores
			projectDir := filepath.Join(storeDir, "projects", projectID)
			doctrineDir := filepath.Join(projectDir, "doctrine")
			playbookDir := filepath.Join(projectDir, "playbook")

			// If the proposal was previously accepted, call RejectAfterAccept to supersede
			if targetProposal.Decision != nil && targetProposal.Decision.Action == "accepted" {
				// Create stores for superseding
				doctrineStore := doctrine.NewFSStore()
				doctrineStore.Dir = doctrineDir
				playbookStore := &playbook.Store{Dir: playbookDir}

				if err := proposaltriage.RejectAfterAccept(
					targetProposal.Decision,
					rejectionDecision,
					doctrineStore,
					playbookStore,
				); err != nil {
					return fmt.Errorf("reject after accept: %w", err)
				}
			} else if targetProposal.Decision != nil {
				// Proposal already has a decision but it's not accepted
				return fmt.Errorf("proposal %q already has a decision: %s", proposalID, targetProposal.Decision.Action)
			}

			// Save the rejection decision to run's evidence directory
			runEvidenceDir := runstore.NewStore(storeDir).RunEvidenceDir(targetProposal.RunID)
			if err := proposaltriage.SaveDecisions(runEvidenceDir, []proposaltriage.Decision{*rejectionDecision}); err != nil {
				return fmt.Errorf("save decision: %w", err)
			}

			// Report results
			fmt.Printf("Proposal %q rejected\n", proposalID)
			fmt.Printf("Reason: %s\n", reason)
			if targetProposal.Decision != nil && targetProposal.Decision.Action == "accepted" {
				fmt.Printf("Note: Previously accepted entry %s marked as superseded\n", targetProposal.Decision.MaterializedID)
			}

			return nil
		},
	}

	cmd.Flags().String("store-dir", "", "Override store directory (default: .gromit-next)")
	cmd.Flags().String("reason", "", "Reason for rejecting the proposal (required)")
	cmd.MarkFlagRequired("reason")

	return cmd
}

// displayPendingProposals renders grouped pending proposals.
// Displays group information (reason, size) followed by proposals in each group.
func displayPendingProposals(groups []proposaltriage.ProposalGroup) error {
	if len(groups) == 0 {
		fmt.Println("No pending proposals found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	for i, group := range groups {
		// Group header with reason and size
		fmt.Fprintf(w, "[Group %d] %s (size: %d)\n", i+1, group.GroupReason, len(group.Proposals))

		// Proposals in this group
		fmt.Fprintln(w, "ID\tTYPE\tRUN\tCONFIDENCE\tTITLE")

		// Sort proposals in group by creation time descending
		sortedProposals := make([]proposaltriage.PendingProposal, len(group.Proposals))
		copy(sortedProposals, group.Proposals)
		sort.Slice(sortedProposals, func(i, j int) bool {
			return sortedProposals[i].CreatedAt.After(sortedProposals[j].CreatedAt)
		})

		for _, p := range sortedProposals {
			if p.Proposal == nil {
				continue
			}

			// Truncate ID for display
			displayID := p.Proposal.ID
			if len(displayID) > 12 {
				displayID = displayID[:12]
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				displayID,
				p.Proposal.Type,
				p.RunID,
				p.Proposal.Confidence,
				p.Proposal.Title,
			)
		}

		// Blank line between groups
		if i < len(groups)-1 {
			fmt.Fprintln(w, "")
		}
	}

	return w.Flush()
}

// displayAllProposals renders all proposals (pending and decided) in a table format.
func displayAllProposals(proposals []proposaltriage.AllProposal) error {
	if len(proposals) == 0 {
		fmt.Println("No proposals found.")
		return nil
	}

	// Sort by creation time descending
	sort.Slice(proposals, func(i, j int) bool {
		return proposals[i].CreatedAt.After(proposals[j].CreatedAt)
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tRUN\tCONFIDENCE\tTITLE\tSTATUS")

	for _, p := range proposals {
		if p.Proposal == nil {
			continue
		}

		// Truncate ID for display
		displayID := p.Proposal.ID
		if len(displayID) > 12 {
			displayID = displayID[:12]
		}

		status := "pending"
		if p.Decision != nil {
			status = p.Decision.Action
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			displayID,
			p.Proposal.Type,
			p.RunID,
			p.Proposal.Confidence,
			p.Proposal.Title,
			status,
		)
	}

	return w.Flush()
}

// displayProposalDetail renders a single proposal with all its details.
func displayProposalDetail(ap *proposaltriage.AllProposal) error {
	p := ap.Proposal
	if p == nil {
		return fmt.Errorf("malformed proposal detail: proposal is missing from distillation file")
	}

	fmt.Println("=== Proposal Detail ===")
	fmt.Printf("ID: %s\n", p.ID)
	fmt.Printf("Type: %s\n", p.Type)
	fmt.Printf("Title: %s\n", p.Title)
	fmt.Printf("Confidence: %s\n", p.Confidence)
	fmt.Println()

	fmt.Println("--- What Happened ---")
	fmt.Println(p.WhatHappened)
	fmt.Println()

	fmt.Println("--- What Was Missing ---")
	fmt.Println(p.WhatWasMissing)
	fmt.Println()

	fmt.Println("--- Proposed Change ---")
	fmt.Println(p.ProposedChange)
	fmt.Println()

	fmt.Println("--- Rationale ---")
	fmt.Println(p.Rationale)
	fmt.Println()

	fmt.Println("--- Confidence Rationale ---")
	fmt.Println(p.ConfidenceRationale)
	fmt.Println()

	if len(p.EvidenceReferences) > 0 {
		fmt.Println("--- Evidence References ---")
		for i, ref := range p.EvidenceReferences {
			fmt.Printf("%d. %s\n", i+1, ref)
		}
		fmt.Println()
	}

	fmt.Println("--- Source Context ---")
	fmt.Printf("Run ID: %s\n", ap.RunID)
	fmt.Printf("Spec ID: %s\n", ap.SpecID)
	fmt.Println()

	if ap.Decision != nil {
		fmt.Println("--- Decision ---")
		fmt.Printf("Status: %s\n", ap.Decision.Action)
		fmt.Printf("Reason: %s\n", ap.Decision.Reason)
		if ap.Decision.ApprovedTitle != "" {
			fmt.Printf("Approved Title: %s\n", ap.Decision.ApprovedTitle)
		}
		if ap.Decision.ApprovedChange != "" {
			fmt.Printf("Approved Change: %s\n", ap.Decision.ApprovedChange)
		}
		if ap.Decision.ApprovedRationale != "" {
			fmt.Printf("Approved Rationale: %s\n", ap.Decision.ApprovedRationale)
		}
		if ap.Decision.MaterializedID != "" {
			fmt.Printf("Materialized ID: %s\n", ap.Decision.MaterializedID)
		}
		if ap.Decision.DuplicateOf != "" {
			fmt.Printf("Duplicate Of: %s\n", ap.Decision.DuplicateOf)
		}
		fmt.Printf("Decided At: %s\n", ap.Decision.DecidedAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("--- Decision ---")
		fmt.Println("Status: pending")
	}

	return nil
}

// loadProjectID attempts to load the project ID from the store configuration.
func loadProjectID(storeDir string) string {
	// Try to load from .gromit-next/projects/ directory
	projectsDir := filepath.Join(storeDir, "projects")
	if entries, err := os.ReadDir(projectsDir); err == nil && len(entries) > 0 {
		// Return the first (and typically only) project directory
		for _, entry := range entries {
			if entry.IsDir() {
				return entry.Name()
			}
		}
	}

	// Try to load from run store metadata
	store := runstore.NewStore(storeDir)
	runs, err := store.List("")
	if err == nil && len(runs) > 0 {
		return runs[0].ProjectID
	}

	return ""
}

func init() {
	proposalsCmd.AddCommand(newReviewProposalsListCmd())
	proposalsCmd.AddCommand(newReviewProposalsShowCmd())
	proposalsCmd.AddCommand(newReviewProposalsAcceptCmd())
	proposalsCmd.AddCommand(newReviewProposalsRejectCmd())
}
