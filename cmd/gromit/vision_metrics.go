package main

import (
	"encoding/json"
	"fmt"

	"github.com/danabrams/gromit/internal/visionmetrics"
	"github.com/spf13/cobra"
)

var visionMetricsCmd = &cobra.Command{
	Use:   "vision-metrics",
	Short: "Manage vision metrics",
	Long:  `Commands for managing and reporting vision metrics.`,
}

var visionMetricsValidateCmd = &cobra.Command{
	Use:   "validate <records-path>",
	Short: "Validate stored vision metrics records",
	Long:  `Load and validate stored vision metrics records, surfacing per-record failures.`,
	Args:  cobra.ExactArgs(1),
	RunE:  visionMetricsValidate,
}

var visionMetricsReportCmd = &cobra.Command{
	Use:   "report <records-path>",
	Short: "Report vision metrics KPI rollup",
	Long:  `Output vision metrics KPI rollup in text and JSON formats.`,
	Args:  cobra.ExactArgs(1),
	RunE:  visionMetricsReport,
}

var visionMetricsReportJSON bool

func init() {
	rootCmd.AddCommand(visionMetricsCmd)
	visionMetricsCmd.AddCommand(visionMetricsValidateCmd)
	visionMetricsCmd.AddCommand(visionMetricsReportCmd)
	visionMetricsReportCmd.Flags().BoolVar(&visionMetricsReportJSON, "json", false, "Output in JSON format")
}

func visionMetricsValidate(cmd *cobra.Command, args []string) error {
	recordsPath := args[0]

	records, err := visionmetrics.LoadRecords(recordsPath)
	if err != nil {
		return fmt.Errorf("loading records: %w", err)
	}

	if len(records) == 0 {
		fmt.Println("No records to validate")
		return nil
	}

	hasErrors := false
	for i, rec := range records {
		errs := visionmetrics.Validate(rec)
		if len(errs) > 0 {
			hasErrors = true
			fmt.Printf("Record %d (spec_id=%q):\n", i, rec.SpecID)
			for _, err := range errs {
				fmt.Printf("  - %s\n", err.Error())
			}
		}
	}

	if hasErrors {
		fmt.Printf("\nValidation completed with %d record(s) containing errors\n", len(records))
	} else {
		fmt.Printf("All %d record(s) are valid\n", len(records))
	}

	return nil
}

func visionMetricsReport(cmd *cobra.Command, args []string) error {
	recordsPath := args[0]

	records, err := visionmetrics.LoadRecords(recordsPath)
	if err != nil {
		return fmt.Errorf("loading records: %w", err)
	}

	rollup := visionmetrics.ComputeRollup(records)

	if visionMetricsReportJSON {
		data, err := json.MarshalIndent(rollup, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling rollup to JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Println("Vision Metrics KPI Rollup")
		fmt.Println("=========================")
		fmt.Printf("Human Tactical Intervention Rate:   %.2f%% (%d/%d)\n",
			rollup.HumanTacticalInterventionRate.Rate*100,
			rollup.HumanTacticalInterventionRate.Numerator,
			rollup.HumanTacticalInterventionRate.Denominator)
		fmt.Printf("Human Debugging Intervention Rate:  %.2f%% (%d/%d)\n",
			rollup.HumanDebuggingInterventionRate.Rate*100,
			rollup.HumanDebuggingInterventionRate.Numerator,
			rollup.HumanDebuggingInterventionRate.Denominator)
		fmt.Printf("First Integration Pass Rate:        %.2f%% (%d/%d)\n",
			rollup.FirstIntegrationPassRate.Rate*100,
			rollup.FirstIntegrationPassRate.Numerator,
			rollup.FirstIntegrationPassRate.Denominator)
		fmt.Printf("Escaped Regression Rate:            %.2f%% (%d/%d)\n",
			rollup.EscapedRegressionRate.Rate*100,
			rollup.EscapedRegressionRate.Numerator,
			rollup.EscapedRegressionRate.Denominator)
		fmt.Printf("Accepted Without Rework Rate:       %.2f%% (%d/%d)\n",
			rollup.AcceptedWithoutReworkRate.Rate*100,
			rollup.AcceptedWithoutReworkRate.Numerator,
			rollup.AcceptedWithoutReworkRate.Denominator)
	}

	return nil
}
