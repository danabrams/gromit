package loop

import (
	"fmt"
	"testing"
)

func TestIntegrationBranchLifecycle_PreservesBranchAndEventLogOnAndonFailure(t *testing.T) {
	t.Parallel()

	assertFailureBranchLifecycle(t, "spec-branch-lifecycle-andon", fmt.Errorf("andon triggered"))
}
