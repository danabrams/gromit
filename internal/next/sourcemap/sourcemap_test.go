package sourcemap

import "testing"

func TestSourceMap_NormalizeNilFields(t *testing.T) {
	var sm SourceMap
	sm.NormalizeNilFields()
	if sm.Entries == nil {
		t.Error("Entries should be initialized, not nil")
	}
}
