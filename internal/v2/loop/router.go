package loop

import "github.com/danabrams/gromit/internal/v2/llmtypes"

// LoopRouter abstracts the Select method used by bead and component routers.
type LoopRouter interface {
	Select(phase, tier string) (llmtypes.LLMProvider, string, string, error)
}
