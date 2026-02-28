package specmerge_test

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/runner/specmerge"
)

func TestPRClient_InterfaceExists(t *testing.T) {
	t.Parallel()

	// Verify PRClient interface exists and has all required methods
	var _ specmerge.PRClient

	// Verify interface has the required methods by checking method signatures
	iface := reflect.TypeOf((*specmerge.PRClient)(nil)).Elem()

	requiredMethods := map[string]bool{
		"CreatePR":         false,
		"GetPR":            false,
		"ListChecks":       false,
		"PostReview":       false,
		"PostComment":      false,
		"RequestReviewers": false,
		"MergePR":          false,
	}

	for i := 0; i < iface.NumMethod(); i++ {
		method := iface.Method(i)
		if _, ok := requiredMethods[method.Name]; ok {
			requiredMethods[method.Name] = true
		}
	}

	for method, found := range requiredMethods {
		if !found {
			t.Errorf("PRClient missing method: %s", method)
		}
	}
}
