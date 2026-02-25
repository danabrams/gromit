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
	return nil
}
