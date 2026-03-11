package evidence

import (
	"os"
	"testing"
)

func TestAssembleBundle_CreatesEvidenceDir(t *testing.T) {
	dir := t.TempDir()
	evidenceDir := dir + "/evidence"
	b := NewBundler(evidenceDir)
	err := b.Init()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(evidenceDir); os.IsNotExist(err) {
		t.Fatal("evidence dir should exist")
	}
}
