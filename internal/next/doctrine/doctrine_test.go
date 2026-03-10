package doctrine

import "testing"

func TestDoctrine_NormalizeNilFields(t *testing.T) {
	var d Doctrine
	d.NormalizeNilFields()
	if d.Rules == nil {
		t.Error("Rules should be initialized, not nil")
	}
}
