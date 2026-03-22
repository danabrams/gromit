package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gromit-next",
	Short: "Gromit Next — workspace-level project memory system",
	Long: `Gromit Next manages project cells outside repositories, inspects projects
to build structured context, and compiles minimal context packets for agent consumption.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(specCmd)
	rootCmd.AddCommand(reviewCmd)
	execCmd.AddCommand(newExecSpecCmd())
	execCmd.AddCommand(newExecShowCmd())
	execCmd.AddCommand(newExecListCmd())
	execCmd.AddCommand(newExecCompleteCmd())
	specCmd.AddCommand(newSpecListCmd())
	reviewCmd.AddCommand(newReviewDistillCmd())
	reviewCmd.AddCommand(proposalsCmd)
}
