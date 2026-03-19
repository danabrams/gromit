package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/spf13/cobra"
)

// newReviewShowCmd creates the `review show` command.
func newReviewShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [run-id]",
		Short: "Show review results for a run",
		Long: `Show review results for a run. Use --details to include linked technical artifacts.
If no run-id is given, the latest run is shown (by modification time of .gromit-next/runs/).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			storeDir, _ := cmd.Flags().GetString("store-dir")
			details, _ := cmd.Flags().GetBool("details")

			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			var runID string
			if len(args) == 0 || args[0] == "latest" {
				id, err := findLatestRunID(storeDir, "", nil)
				if err != nil {
					return err
				}
				runID = id
			} else {
				runID = args[0]
			}

			output, err := reviewShow(runID, storeDir, details)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), output)
			return nil
		},
	}
	cmd.Flags().Bool("details", false, "Include linked technical artifacts (validation.json, acceptance.json, review.json)")
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	return cmd
}

// reviewShow formats review results as a human-readable string.
func reviewShow(runID string, storeDir string, details bool) (string, error) {
	// Load run and ensure review packet exists
	_, err := loadRunAndEnsurePacket(runID, storeDir)
	if err != nil {
		return "", err
	}

	var b strings.Builder

	// Get evidence directory
	evidenceDir := filepath.Join(storeDir, "runs", runID, "evidence")

	// Read product-review.json
	productReviewPath := filepath.Join(evidenceDir, "product-review.json")
	productData, err := os.ReadFile(productReviewPath)
	if err != nil {
		return "", fmt.Errorf("read product-review.json: %w", err)
	}

	var productReview reviewpacket.ProductReview
	if err := json.Unmarshal(productData, &productReview); err != nil {
		return "", fmt.Errorf("parse product-review.json: %w", err)
	}

	// Read process-review.json
	processReviewPath := filepath.Join(evidenceDir, "process-review.json")
	processData, err := os.ReadFile(processReviewPath)
	if err != nil {
		return "", fmt.Errorf("read process-review.json: %w", err)
	}

	var processReview struct {
		TrustLevel string `json:"trust_level"`
	}
	if err := json.Unmarshal(processData, &processReview); err != nil {
		return "", fmt.Errorf("parse process-review.json: %w", err)
	}

	// Render product review
	rendered := reviewpacket.RenderProductReview(productReview)
	b.WriteString(rendered)

	// Add trust banner
	b.WriteString("\n---\n\n")
	b.WriteString(fmt.Sprintf("**Trust Level:** %s\n\n", processReview.TrustLevel))

	// Include technical artifacts if requested
	if details {
		b.WriteString("---\n\n## Technical Artifacts\n\n")

		// Include validation.json
		validationPath := filepath.Join(evidenceDir, "validation.json")
		if data, err := os.ReadFile(validationPath); err == nil {
			b.WriteString("### validation.json\n\n```json\n")
			b.Write(data)
			b.WriteString("\n```\n\n")
		}

		// Include acceptance.json
		acceptancePath := filepath.Join(evidenceDir, "acceptance.json")
		if data, err := os.ReadFile(acceptancePath); err == nil {
			b.WriteString("### acceptance.json\n\n```json\n")
			b.Write(data)
			b.WriteString("\n```\n\n")
		}

		// Include review.json
		reviewPath := filepath.Join(evidenceDir, "review.json")
		if data, err := os.ReadFile(reviewPath); err == nil {
			b.WriteString("### review.json\n\n```json\n")
			b.Write(data)
			b.WriteString("\n```\n\n")
		}
	}

	return b.String(), nil
}

func init() {
	reviewCmd.AddCommand(newReviewShowCmd())
}
