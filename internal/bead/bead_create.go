package bead

import (
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/jsonutil"
)

func validateReferencedBeadID(id, idType string) error {
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return fmt.Errorf("invalid %s %q", idType, id)
	}
	return nil
}

// Create creates a new bead via the bd CLI and returns the created bead.
func (c *Client) Create(title string, priority int, labels []string, expectedOutputs []string) (*Bead, error) {
	return c.CreateWithParentAndDescription(title, priority, labels, expectedOutputs, "", "")
}

// CreateWithParent creates a new bead with an optional parent and description via the bd CLI.
func (c *Client) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*Bead, error) {
	return c.CreateWithParentAndDescription(title, priority, labels, expectedOutputs, parentID, "")
}

// CreateWithParentAndDescription creates a new bead with an optional parent and description via the bd CLI.
func (c *Client) CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}

	// Add parent if specified.
	var extra []string
	if parentID != "" {
		if err := validateReferencedBeadID(parentID, "parent ID"); err != nil {
			return nil, err
		}
		extra = append(extra, "--parent", parentID)
	}

	return c.runCreate(title, priority, labels, expectedOutputs, description, extra)
}

// CreateWithDepsAndDescription creates a new bead with dependencies and description via the bd CLI.
func (c *Client) CreateWithDepsAndDescription(title string, priority int, labels []string, expectedOutputs []string, dependencies []string, description string) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}

	// Add dependencies if specified.
	var extra []string
	if len(dependencies) > 0 {
		// Validate all dependency IDs.
		for _, depID := range dependencies {
			if err := validateReferencedBeadID(depID, "dependency ID"); err != nil {
				return nil, err
			}
		}
		// bd --deps accepts comma-separated list or multiple IDs.
		extra = append(extra, "--deps", strings.Join(dependencies, ","))
	}

	return c.runCreate(title, priority, labels, expectedOutputs, description, extra)
}

// runCreate builds args, writes description content to a temp file, invokes bd create, and parses the result.
// extraArgs are inserted after acceptance (e.g. --parent or --deps flags).
func (c *Client) runCreate(title string, priority int, labels []string, expectedOutputs []string, description string, extraArgs []string) (*Bead, error) {
	args := []string{"create", title, "--priority", fmt.Sprintf("%d", priority), "--json"}

	for _, label := range labels {
		args = append(args, "--label", label)
	}

	if len(expectedOutputs) > 0 {
		acceptance := strings.Join(expectedOutputs, "\n")
		args = append(args, "--acceptance", acceptance)
	}

	args = append(args, extraArgs...)

	if description != "" {
		descriptionPath, cleanup, err := writeTempFile("bd-description-*.md", description)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		args = append(args, "--body-file", descriptionPath)
	}

	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("bd create: %w", err)
	}

	var b Bead
	if err := jsonutil.ExtractObject(out, &b); err != nil {
		return nil, fmt.Errorf("parsing bd create output: %w", err)
	}

	if err := prepareBeadForUse(&b); err != nil {
		return nil, fmt.Errorf("invalid bead data: %w", err)
	}

	return &b, nil
}
