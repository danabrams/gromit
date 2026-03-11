package runstore

import "testing"

func TestRunState_IsTerminal(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"running", false},
		{"ready_for_review", true},
		{"needs_human", true},
		{"blocked", true},
		{"completed", true},
	}
	for _, tc := range cases {
		rs := RunState{Status: tc.status}
		if rs.IsTerminal() != tc.want {
			t.Errorf("status=%s: want IsTerminal=%v", tc.status, tc.want)
		}
	}
}

func TestNewRunState_GeneratesID(t *testing.T) {
	rs := NewRunState("spec-001", "proj-1")
	if rs.RunID == "" {
		t.Fatal("RunID must not be empty")
	}
	if rs.Status != "running" {
		t.Fatalf("want running, got %s", rs.Status)
	}
}

func TestNewRunState_TasksNotNil(t *testing.T) {
	rs := NewRunState("spec-001", "proj-1")
	if rs.Tasks == nil {
		t.Fatal("Tasks must not be nil")
	}
}

func TestRunState_NormalizeNilFields(t *testing.T) {
	rs := &RunState{}
	rs.NormalizeNilFields()
	if rs.Tasks == nil {
		t.Fatal("NormalizeNilFields must set Tasks to empty slice")
	}
}

func TestTask_NormalizeNilFields(t *testing.T) {
	tk := &Task{}
	tk.NormalizeNilFields()
	if tk.ExpectedTouchedArea == nil {
		t.Fatal("ExpectedTouchedArea must not be nil after normalize")
	}
	if tk.ProofChecks == nil {
		t.Fatal("ProofChecks must not be nil after normalize")
	}
	if tk.FilesChanged == nil {
		t.Fatal("FilesChanged must not be nil after normalize")
	}
	if tk.FailuresAddressed == nil {
		t.Fatal("FailuresAddressed must not be nil after normalize")
	}
}
