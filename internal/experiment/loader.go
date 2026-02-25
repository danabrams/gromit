package experiment

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadExperiments returns the experiments defined in the given directory.
func LoadExperiments(dir string) ([]*Experiment, error) {
	pattern := filepath.Join(dir, "*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var exps []*Experiment
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		var exp Experiment
		if err := yaml.Unmarshal(data, &exp); err != nil {
			return nil, err
		}

		if err := validateExperiment(&exp); err != nil {
			return nil, fmt.Errorf("validating %s: %w", path, err)
		}

		exps = append(exps, &exp)
	}

	return exps, nil
}

func validateExperiment(exp *Experiment) error {
	if !ValidPhases[exp.Phase] {
		return fmt.Errorf("invalid phase %q", exp.Phase)
	}
	if exp.Control == nil {
		return fmt.Errorf("experiment %q missing control variant", exp.ID)
	}
	if exp.Control.ID == "" {
		return fmt.Errorf("control variant in experiment %q missing ID", exp.ID)
	}
	ids := make(map[string]struct{})
	ids[exp.Control.ID] = struct{}{}
	for _, variant := range exp.Variants {
		if variant == nil {
			return fmt.Errorf("nil variant in experiment %q", exp.ID)
		}
		if variant.ID == "" {
			return fmt.Errorf("variant in experiment %q missing ID", exp.ID)
		}
		if _, exists := ids[variant.ID]; exists {
			return fmt.Errorf("duplicate variant ID %q", variant.ID)
		}
		ids[variant.ID] = struct{}{}
	}
	return nil
}
