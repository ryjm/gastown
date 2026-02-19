package cmd

import (
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
)

func setupBootstrapTestRegistry(t *testing.T) {
	t.Helper()
	reg := session.NewPrefixRegistry()
	reg.Register("gt", "gastown")
	old := session.DefaultRegistry()
	session.SetDefaultRegistry(reg)
	t.Cleanup(func() { session.SetDefaultRegistry(old) })
}

func codexRuntimeConfigForBootstrapTests() *config.RuntimeConfig {
	return &config.RuntimeConfig{
		Provider:   "codex",
		PromptMode: "none",
		Hooks: &config.RuntimeHooksConfig{
			Provider: "none",
		},
		Tmux: &config.RuntimeTmuxConfig{
			ReadyDelayMs: 3000,
		},
	}
}

func TestRuntimeConfigForSessionStartupBootstrap_RoleMapping(t *testing.T) {
	setupBootstrapTestRegistry(t)

	townRoot := t.TempDir()
	tests := []struct {
		name        string
		sessionName string
		wantRole    string
	}{
		{name: "crew", sessionName: "gt-crew-max", wantRole: "crew"},
		{name: "polecat", sessionName: "gt-toast", wantRole: "polecat"},
		{name: "witness", sessionName: "gt-witness", wantRole: "witness"},
		{name: "refinery", sessionName: "gt-refinery", wantRole: "refinery"},
		{name: "boot", sessionName: "hq-boot", wantRole: "boot"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRole, gotConfig, err := runtimeConfigForSessionStartupBootstrap(tt.sessionName, townRoot)
			if err != nil {
				t.Fatalf("runtimeConfigForSessionStartupBootstrap(%q): %v", tt.sessionName, err)
			}
			if gotRole != tt.wantRole {
				t.Fatalf("role = %q, want %q", gotRole, tt.wantRole)
			}
			if gotConfig == nil {
				t.Fatal("runtime config should not be nil")
			}
		})
	}
}

func TestRunRespawnStartupBootstrap_CodexCrew(t *testing.T) {
	origSleep := startupFallbackSleep
	origRun := startupFallbackRun
	defer func() {
		startupFallbackSleep = origSleep
		startupFallbackRun = origRun
	}()

	var slept bool
	var gotSession string
	var gotRole string
	var ran bool

	startupFallbackSleep = func(*config.RuntimeConfig) {
		slept = true
	}
	startupFallbackRun = func(_ *tmux.Tmux, sessionID, role string, _ *config.RuntimeConfig) error {
		ran = true
		gotSession = sessionID
		gotRole = role
		return nil
	}

	err := runRespawnStartupBootstrap(nil, "gt-crew-max", "crew", codexRuntimeConfigForBootstrapTests())
	if err != nil {
		t.Fatalf("runRespawnStartupBootstrap: %v", err)
	}
	if !slept {
		t.Fatal("expected ready delay sleep before fallback nudge")
	}
	if !ran {
		t.Fatal("expected startup fallback nudge to run")
	}
	if gotSession != "gt-crew-max" {
		t.Fatalf("session = %q, want gt-crew-max", gotSession)
	}
	if gotRole != "crew" {
		t.Fatalf("role = %q, want crew", gotRole)
	}
}

func TestRunRespawnStartupBootstrap_SkipsWhenHooksHandleStartup(t *testing.T) {
	origSleep := startupFallbackSleep
	origRun := startupFallbackRun
	defer func() {
		startupFallbackSleep = origSleep
		startupFallbackRun = origRun
	}()

	slept := false
	ran := false
	startupFallbackSleep = func(*config.RuntimeConfig) { slept = true }
	startupFallbackRun = func(_ *tmux.Tmux, _, _ string, _ *config.RuntimeConfig) error {
		ran = true
		return nil
	}

	err := runRespawnStartupBootstrap(nil, "gt-crew-max", "crew", &config.RuntimeConfig{
		Provider: "claude",
		Hooks: &config.RuntimeHooksConfig{
			Provider: "claude",
		},
	})
	if err != nil {
		t.Fatalf("runRespawnStartupBootstrap: %v", err)
	}
	if slept {
		t.Fatal("should not sleep when fallback is not needed")
	}
	if ran {
		t.Fatal("should not run fallback when hooks handle startup")
	}
}

func TestScheduleRespawnStartupBootstrap_CodexCrew(t *testing.T) {
	orig := tmuxRunShellBackground
	defer func() { tmuxRunShellBackground = orig }()

	var scripts []string
	tmuxRunShellBackground = func(script string) error {
		scripts = append(scripts, script)
		return nil
	}

	err := scheduleRespawnStartupBootstrap("gt-crew-max", "crew", codexRuntimeConfigForBootstrapTests())
	if err != nil {
		t.Fatalf("scheduleRespawnStartupBootstrap: %v", err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 scheduled script, got %d", len(scripts))
	}
	if !strings.Contains(scripts[0], "sleep 3.000") {
		t.Fatalf("expected ready-delay sleep in script, got %q", scripts[0])
	}
	if !strings.Contains(scripts[0], "gt prime") {
		t.Fatalf("expected gt prime command in script, got %q", scripts[0])
	}
	if strings.Contains(scripts[0], "mail check --inject") {
		t.Fatalf("crew bootstrap should not inject mail, got %q", scripts[0])
	}
}

func TestScheduleRespawnStartupBootstrap_CodexPolecat(t *testing.T) {
	orig := tmuxRunShellBackground
	defer func() { tmuxRunShellBackground = orig }()

	var scripts []string
	tmuxRunShellBackground = func(script string) error {
		scripts = append(scripts, script)
		return nil
	}

	err := scheduleRespawnStartupBootstrap("gt-toast", "polecat", codexRuntimeConfigForBootstrapTests())
	if err != nil {
		t.Fatalf("scheduleRespawnStartupBootstrap: %v", err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 scheduled script, got %d", len(scripts))
	}
	if !strings.Contains(scripts[0], "gt prime && gt mail check --inject") {
		t.Fatalf("expected autonomous bootstrap command, got %q", scripts[0])
	}
}
