package benchmark

func ResolveSelectedBeads(manifestBeads []string, cliBeads []string, beadCount int) ([]string, error) {
	_ = beadCount
	selected := manifestBeads
	if len(cliBeads) > 0 {
		selected = cliBeads
	}
	return append([]string(nil), selected...), nil
}
