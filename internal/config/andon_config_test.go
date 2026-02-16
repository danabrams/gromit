package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetDefaultsAppliesAndonThresholdDefaults(t *testing.T) {
	// Expected failure: AndonConfig and default Andon constants are not implemented on Config yet.
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Andon.AssumptionBudget != DefaultAndonAssumptionBudget {
		t.Fatalf("Andon.AssumptionBudget = %d, want %d", cfg.Andon.AssumptionBudget, DefaultAndonAssumptionBudget)
	}
	if cfg.Andon.L1RetryCap != DefaultAndonL1RetryCap {
		t.Fatalf("Andon.L1RetryCap = %d, want %d", cfg.Andon.L1RetryCap, DefaultAndonL1RetryCap)
	}
	if cfg.Andon.L1TimeCapMinutes != DefaultAndonL1TimeCapMinutes {
		t.Fatalf("Andon.L1TimeCapMinutes = %d, want %d", cfg.Andon.L1TimeCapMinutes, DefaultAndonL1TimeCapMinutes)
	}
	if cfg.Andon.L2TimeCapMinutes != DefaultAndonL2TimeCapMinutes {
		t.Fatalf("Andon.L2TimeCapMinutes = %d, want %d", cfg.Andon.L2TimeCapMinutes, DefaultAndonL2TimeCapMinutes)
	}

	if !cfg.Andon.HardStops.BlockBulkDelete {
		t.Fatal("Andon.HardStops.BlockBulkDelete = false, want true")
	}
	if !cfg.Andon.HardStops.BlockIrreversibleMigrations {
		t.Fatal("Andon.HardStops.BlockIrreversibleMigrations = false, want true")
	}
	if !cfg.Andon.HardStops.BlockCredentialChanges {
		t.Fatal("Andon.HardStops.BlockCredentialChanges = false, want true")
	}
	if len(cfg.Andon.HardStops.BulkDeleteAllowlist) == 0 {
		t.Fatal("Andon.HardStops.BulkDeleteAllowlist is empty, want documented defaults")
	}
}

func TestLoadAndonConfigOverridesFromYAML(t *testing.T) {
	// Expected failure: AndonConfig YAML surface is not wired into Load()/yaml parsing yet.
	yamlContent := `
andon:
  assumption_budget: 3
  l1_retry_cap: 4
  l1_time_cap_minutes: 6
  l2_time_cap_minutes: 20
  hard_stops:
    block_bulk_delete: false
    block_irreversible_migrations: true
    block_credential_changes: false
    bulk_delete_allowlist:
      - ".gromit/logs/**"
      - "test/fixtures/**"
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Andon.AssumptionBudget != 3 {
		t.Fatalf("Andon.AssumptionBudget = %d, want 3", cfg.Andon.AssumptionBudget)
	}
	if cfg.Andon.L1RetryCap != 4 {
		t.Fatalf("Andon.L1RetryCap = %d, want 4", cfg.Andon.L1RetryCap)
	}
	if cfg.Andon.L1TimeCapMinutes != 6 {
		t.Fatalf("Andon.L1TimeCapMinutes = %d, want 6", cfg.Andon.L1TimeCapMinutes)
	}
	if cfg.Andon.L2TimeCapMinutes != 20 {
		t.Fatalf("Andon.L2TimeCapMinutes = %d, want 20", cfg.Andon.L2TimeCapMinutes)
	}

	if cfg.Andon.HardStops.BlockBulkDelete {
		t.Fatal("Andon.HardStops.BlockBulkDelete = true, want false")
	}
	if !cfg.Andon.HardStops.BlockIrreversibleMigrations {
		t.Fatal("Andon.HardStops.BlockIrreversibleMigrations = false, want true")
	}
	if cfg.Andon.HardStops.BlockCredentialChanges {
		t.Fatal("Andon.HardStops.BlockCredentialChanges = true, want false")
	}

	wantAllowlist := []string{".gromit/logs/**", "test/fixtures/**"}
	if len(cfg.Andon.HardStops.BulkDeleteAllowlist) != len(wantAllowlist) {
		t.Fatalf("len(Andon.HardStops.BulkDeleteAllowlist) = %d, want %d", len(cfg.Andon.HardStops.BulkDeleteAllowlist), len(wantAllowlist))
	}
	for i, want := range wantAllowlist {
		if cfg.Andon.HardStops.BulkDeleteAllowlist[i] != want {
			t.Fatalf("Andon.HardStops.BulkDeleteAllowlist[%d] = %q, want %q", i, cfg.Andon.HardStops.BulkDeleteAllowlist[i], want)
		}
	}
}

func TestGromitYAMLDocumentsAndonConfig(t *testing.T) {
	// Expected failure: DefaultAndonConfigDocSectionTitle constant and Andon docs are not implemented yet.
	if DefaultAndonConfigDocSectionTitle == "" {
		t.Fatal("DefaultAndonConfigDocSectionTitle must be non-empty")
	}

	// Expected failure: gromit.yaml does not document the Andon config section and conventions yet.
	projectRoot := findProjectRoot(t)
	cfgPath := filepath.Join(projectRoot, "gromit.yaml")

	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", cfgPath, err)
	}
	text := string(content)

	t.Run("documents_andon_section", func(t *testing.T) {
		if !strings.Contains(text, DefaultAndonConfigDocSectionTitle) {
			t.Fatal("gromit.yaml missing andon section")
		}
	})

	t.Run("documents_assumption_budget", func(t *testing.T) {
		if !strings.Contains(text, "assumption_budget:") {
			t.Fatal("gromit.yaml missing andon.assumption_budget field")
		}
		if !strings.Contains(strings.ToLower(text), "assumption") {
			t.Fatal("gromit.yaml missing assumption budget intent comment")
		}
	})

	t.Run("documents_l1_l2_caps", func(t *testing.T) {
		if !strings.Contains(text, "l1_retry_cap:") {
			t.Fatal("gromit.yaml missing andon.l1_retry_cap field")
		}
		if !strings.Contains(text, "l1_time_cap_minutes:") {
			t.Fatal("gromit.yaml missing andon.l1_time_cap_minutes field")
		}
		if !strings.Contains(text, "l2_time_cap_minutes:") {
			t.Fatal("gromit.yaml missing andon.l2_time_cap_minutes field")
		}
	})

	t.Run("documents_hard_stop_allowlist_conventions", func(t *testing.T) {
		if !strings.Contains(text, "hard_stops:") {
			t.Fatal("gromit.yaml missing andon.hard_stops section")
		}
		if !strings.Contains(text, "bulk_delete_allowlist:") {
			t.Fatal("gromit.yaml missing andon hard-stop allowlist field")
		}
		if !strings.Contains(strings.ToLower(text), "allowlist") {
			t.Fatal("gromit.yaml missing allowlist convention comment")
		}
	})
}
