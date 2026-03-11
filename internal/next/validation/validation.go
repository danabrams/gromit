package validation

import (
	"encoding/json"
	"fmt"
)

type Kind int

const (
	Test Kind = iota
	Lint
	Build
)

func (k Kind) String() string {
	switch k {
	case Test:
		return "test"
	case Lint:
		return "lint"
	case Build:
		return "build"
	default:
		return "unknown"
	}
}

func (k Kind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

func (k *Kind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "test":
		*k = Test
	case "lint":
		*k = Lint
	case "build":
		*k = Build
	default:
		return fmt.Errorf("unknown kind: %q", s)
	}
	return nil
}

type Command struct {
	Name   string `json:"name"`
	Kind   Kind   `json:"kind"`
	Run    string `json:"run"`
	Source string `json:"source"`
}

type CommandSet struct {
	Commands []Command `json:"commands"`
}

func NewCommandSet() CommandSet {
	return CommandSet{Commands: []Command{}}
}

func (cs *CommandSet) Add(cmd Command) {
	cs.Commands = append(cs.Commands, cmd)
}

// NormalizeNilFields maps nil slices/maps to empty values.
// See CLAUDE.md nil-field normalization visibility convention.
func (cs *CommandSet) NormalizeNilFields() {
	if cs.Commands == nil {
		cs.Commands = []Command{}
	}
}

func (cs *CommandSet) ByKind(kind Kind) []Command {
	var result []Command
	for _, cmd := range cs.Commands {
		if cmd.Kind == kind {
			result = append(result, cmd)
		}
	}
	return result
}
