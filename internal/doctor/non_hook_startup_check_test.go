package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

func TestNonHookStartupParityCheck_PassesWithMixedRuntimes(t *testing.T) {
	townRoot, rigPath := setupNonHookParityTown(t)

	townSettings := config.NewTownSettings()
	townSettings.RoleAgents["polecat"] = "codex"
	townSettings.RoleAgents["witness"] = "claude"
	if err := config.SaveTownSettings(config.TownSettingsPath(townRoot), townSettings); err != nil {
		t.Fatalf("SaveTownSettings: %v", err)
	}

	if err := config.SaveRigSettings(config.RigSettingsPath(rigPath), config.NewRigSettings()); err != nil {
		t.Fatalf("SaveRigSettings: %v", err)
	}

	check := NewNonHookStartupParityCheck()
	result := check.Run(&CheckContext{TownRoot: townRoot})

	if result.Status != StatusOK {
		t.Fatalf("status = %v, want %v (details: %v)", result.Status, StatusOK, result.Details)
	}
	if len(result.Details) != 0 {
		t.Fatalf("expected no details, got: %v", result.Details)
	}
	if !strings.Contains(result.Message, "Validated non-hook startup parity") {
		t.Fatalf("expected validation message, got %q", result.Message)
	}
}

func TestNonHookStartupParityCheck_FailsOnIncoherentNonHookHooksConfig(t *testing.T) {
	townRoot, rigPath := setupNonHookParityTown(t)

	townSettings := config.NewTownSettings()
	townSettings.RoleAgents["polecat"] = "codex-broken"
	townSettings.Agents["codex-broken"] = &config.RuntimeConfig{
		Provider:   "codex",
		Command:    "codex",
		PromptMode: "none",
		Hooks: &config.RuntimeHooksConfig{
			Provider:     "claude",
			Dir:          ".claude",
			SettingsFile: "settings.json",
		},
	}
	if err := config.SaveTownSettings(config.TownSettingsPath(townRoot), townSettings); err != nil {
		t.Fatalf("SaveTownSettings: %v", err)
	}

	if err := config.SaveRigSettings(config.RigSettingsPath(rigPath), config.NewRigSettings()); err != nil {
		t.Fatalf("SaveRigSettings: %v", err)
	}

	check := NewNonHookStartupParityCheck()
	result := check.Run(&CheckContext{TownRoot: townRoot})

	if result.Status != StatusError {
		t.Fatalf("status = %v, want %v", result.Status, StatusError)
	}
	if len(result.Details) == 0 {
		t.Fatal("expected failure details")
	}
	if !containsSubstring(result.Details, "provider \"codex\" is non-hook") {
		t.Fatalf("expected codex non-hook mismatch detail, got: %v", result.Details)
	}
	if !strings.Contains(result.FixHint, "hooks.provider") {
		t.Fatalf("expected fix hint to mention hooks.provider, got %q", result.FixHint)
	}
	if !strings.Contains(result.FixHint, "gt mail check --inject") {
		t.Fatalf("expected fix hint to mention mail injection, got %q", result.FixHint)
	}
}

func TestDoctor_AutoRegistersNonHookStartupParityCheck(t *testing.T) {
	townRoot, _ := setupNonHookParityTown(t)

	d := NewDoctor()
	d.Register(&namedNoopCheck{name: "global-state"})

	report := d.Run(&CheckContext{TownRoot: townRoot})
	if !reportContainsCheck(report, "non-hook-startup-parity") {
		t.Fatalf("expected report to include non-hook-startup-parity, got checks: %v", reportCheckNames(report))
	}
}

func setupNonHookParityTown(t *testing.T) (string, string) {
	t.Helper()

	townRoot := t.TempDir()
	rigPath := filepath.Join(townRoot, "testrig")

	dirs := []string{
		filepath.Join(rigPath, "settings"),
		filepath.Join(rigPath, "crew"),
		filepath.Join(rigPath, "polecats"),
		filepath.Join(rigPath, "witness"),
		filepath.Join(rigPath, "refinery"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}

	return townRoot, rigPath
}

func containsSubstring(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(item, needle) {
			return true
		}
	}
	return false
}

func reportContainsCheck(report *Report, checkName string) bool {
	for _, check := range report.Checks {
		if check.Name == checkName {
			return true
		}
	}
	return false
}

func reportCheckNames(report *Report) []string {
	names := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		names = append(names, check.Name)
	}
	return names
}

type namedNoopCheck struct {
	name string
}

func (c *namedNoopCheck) Name() string {
	return c.name
}

func (c *namedNoopCheck) Description() string {
	return "noop"
}

func (c *namedNoopCheck) Run(ctx *CheckContext) *CheckResult {
	return &CheckResult{Name: c.name, Status: StatusOK}
}

func (c *namedNoopCheck) Fix(ctx *CheckContext) error {
	return ErrCannotFix
}

func (c *namedNoopCheck) CanFix() bool {
	return false
}
