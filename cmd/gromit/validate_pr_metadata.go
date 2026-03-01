package main

import (
	"fmt"
	"os"

	"github.com/danabrams/gromit/internal/visionmetrics"
)

// ValidatePRMetadata parses and validates PR metadata from PR body.
// Returns a slice of formatted error messages.
func ValidatePRMetadata(prBody string) []string {
	rec, err := visionmetrics.ParseFromPRBody(prBody)
	if err != nil {
		return []string{fmt.Sprintf("parsing error: %v", err)}
	}

	validationErrs := visionmetrics.Validate(rec)
	var messages []string
	for _, verr := range validationErrs {
		messages = append(messages, verr.Error())
	}
	return messages
}

// runValidatePRMetadataCommand reads PR body from environment and validates it.
// Returns exit code 0 on success, 1 on validation failure.
func runValidatePRMetadataCommand() int {
	prBody := os.Getenv("PR_BODY")
	if prBody == "" {
		fmt.Fprintf(os.Stderr, "error: PR_BODY environment variable not set\n")
		return 1
	}

	errs := ValidatePRMetadata(prBody)
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "PR metadata validation failed:\n")
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", err)
		}
		return 1
	}

	fmt.Printf("PR metadata validation passed\n")
	return 0
}
