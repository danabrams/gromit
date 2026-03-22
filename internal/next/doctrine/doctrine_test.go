package doctrine

import "testing"

func TestDoctrine_NormalizeNilFields(t *testing.T) {
	var d Doctrine
	d.NormalizeNilFields()
	if d.Rules == nil {
		t.Error("Rules should be initialized, not nil")
	}
}

func TestFSStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFSStore()
	store.Dir = dir

	original := Doctrine{
		Rules: []Rule{
			NewRule("no-globals", "Avoid global state", "*"),
			NewRule("test-tables", "Use table-driven tests", "tests"),
		},
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Rules) != len(original.Rules) {
		t.Fatalf("expected %d rules, got %d", len(original.Rules), len(loaded.Rules))
	}

	for i, want := range original.Rules {
		got := loaded.Rules[i]
		if got.ID != want.ID {
			t.Errorf("rule[%d].ID = %q, want %q", i, got.ID, want.ID)
		}
		if got.Summary != want.Summary {
			t.Errorf("rule[%d].Summary = %q, want %q", i, got.Summary, want.Summary)
		}
		if got.Scope != want.Scope {
			t.Errorf("rule[%d].Scope = %q, want %q", i, got.Scope, want.Scope)
		}
		if got.Source != want.Source {
			t.Errorf("rule[%d].Source = %q, want %q", i, got.Source, want.Source)
		}
	}
}

func TestFSStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewFSStore()
	store.Dir = dir

	d, err := store.Load()
	if err != nil {
		t.Fatalf("Load from empty dir: %v", err)
	}
	if d.Rules == nil {
		t.Error("Rules should be non-nil empty slice, got nil")
	}
	if len(d.Rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(d.Rules))
	}
}

func TestRule_SourceAlwaysDeclared(t *testing.T) {
	cases := []struct {
		id      string
		summary string
		scope   string
	}{
		{"r1", "First rule", "*"},
		{"r2", "Second rule", "tests"},
		{"r3", "Third rule", "api"},
	}

	for _, tc := range cases {
		r := NewRule(tc.id, tc.summary, tc.scope)
		if r.Source != "declared" {
			t.Errorf("NewRule(%q).Source = %q, want %q", tc.id, r.Source, "declared")
		}
	}
}
