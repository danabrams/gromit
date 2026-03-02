package visionmetrics

import "testing"

func TestEscapedRegressionStatusValidValues(t *testing.T) {
    statuses := []EscapedRegressionStatus{
        EscapedRegressionYes,
        EscapedRegressionNo,
        EscapedRegressionPending,
    }
    for _, status := range statuses {
        if !status.Valid() {
            t.Errorf("EscapedRegressionStatus %q should be valid", status)
        }
    }
}
