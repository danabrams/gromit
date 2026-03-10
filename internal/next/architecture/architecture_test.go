package architecture

import "testing"

func TestModule_String(t *testing.T) {
	m := Module{
		Name:        "internal/auth",
		Description: "Authentication and authorization",
		Language:    "go",
	}
	if m.Name != "internal/auth" {
		t.Errorf("Name = %q, want %q", m.Name, "internal/auth")
	}
}

func TestDependency_Directions(t *testing.T) {
	d := Dependency{
		From: "internal/auth",
		To:   "internal/config",
		Kind: "import",
	}
	if d.From != "internal/auth" || d.To != "internal/config" {
		t.Errorf("unexpected dependency: %+v", d)
	}
}

func TestArchitecture_AddModule(t *testing.T) {
	arch := New()
	arch.AddModule(Module{Name: "cmd/api", Description: "HTTP entrypoint", Language: "go"})
	arch.AddModule(Module{Name: "internal/core", Description: "Domain logic", Language: "go"})

	if len(arch.Modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(arch.Modules))
	}
}

func TestArchitecture_AddDependency(t *testing.T) {
	arch := New()
	arch.AddDependency(Dependency{From: "cmd/api", To: "internal/core", Kind: "import"})

	if len(arch.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(arch.Dependencies))
	}
}

func TestArchitecture_AddComponent(t *testing.T) {
	arch := New()
	arch.AddComponent(Component{
		Name:        "auth-system",
		Description: "Authentication subsystem",
		Modules:     []string{"internal/auth", "internal/token"},
	})

	if len(arch.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(arch.Components))
	}
	if arch.Components[0].Name != "auth-system" {
		t.Errorf("Name = %q, want %q", arch.Components[0].Name, "auth-system")
	}
	if len(arch.Components[0].Modules) != 2 {
		t.Errorf("expected 2 modules in component, got %d", len(arch.Components[0].Modules))
	}
}

func TestNew_InitializesEmptySlices(t *testing.T) {
	arch := New()
	if arch.Modules == nil {
		t.Error("Modules should be initialized, not nil")
	}
	if arch.Dependencies == nil {
		t.Error("Dependencies should be initialized, not nil")
	}
	if arch.Components == nil {
		t.Error("Components should be initialized, not nil")
	}
}
