package escalation

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestHandlerFailureAPIsUseProviderResult(t *testing.T) {
	var _ func(*Handler, context.Context, *runtypes.BeadContext, *provider.Result) bool = (*Handler).HandleEscalation
	var _ func(*Handler, context.Context, *runtypes.BeadContext, *provider.Result) bool = (*Handler).AnalyzeAndHandleFailure
}
