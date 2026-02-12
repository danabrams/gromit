package pipeline

import "testing"

func TestNew_ReturnsNonNil(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}

	p := New(deps, paths)

	if p == nil {
		t.Fatal("New() returned nil, expected non-nil Pipeline")
	}
}

func TestPaths_FieldAccess(t *testing.T) {
	paths := Paths{
		GromitDir: "/tmp/.gromit",
		SpecsDir:  "/tmp/.gromit/specs",
		PlansDir:  "/tmp/.gromit/plans",
		EpicsDir:  "/tmp/.gromit/epics",
	}

	if paths.GromitDir != "/tmp/.gromit" {
		t.Errorf("GromitDir = %q, want %q", paths.GromitDir, "/tmp/.gromit")
	}
	if paths.SpecsDir != "/tmp/.gromit/specs" {
		t.Errorf("SpecsDir = %q, want %q", paths.SpecsDir, "/tmp/.gromit/specs")
	}
	if paths.PlansDir != "/tmp/.gromit/plans" {
		t.Errorf("PlansDir = %q, want %q", paths.PlansDir, "/tmp/.gromit/plans")
	}
	if paths.EpicsDir != "/tmp/.gromit/epics" {
		t.Errorf("EpicsDir = %q, want %q", paths.EpicsDir, "/tmp/.gromit/epics")
	}
}
