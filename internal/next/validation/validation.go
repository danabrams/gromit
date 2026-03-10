package validation

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
