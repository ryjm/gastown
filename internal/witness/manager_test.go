package witness

import (
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/config"
)

func TestBuildWitnessStartCommand_UsesRoleConfig(t *testing.T) {
	roleConfig := &beads.RoleConfig{
		StartCommand: "exec run --town {town} --rig {rig} --role {role}",
	}

	got, err := buildWitnessStartCommand("/town/rig", "gastown", "/town", "", roleConfig)
	if err != nil {
		t.Fatalf("buildWitnessStartCommand: %v", err)
	}

	want := "exec run --town /town --rig gastown --role witness"
	if got != want {
		t.Errorf("buildWitnessStartCommand = %q, want %q", got, want)
	}
}

func TestBuildWitnessStartCommand_DefaultsToRuntime(t *testing.T) {
	got, err := buildWitnessStartCommand("/town/rig", "gastown", "/town", "", nil)
	if err != nil {
		t.Fatalf("buildWitnessStartCommand: %v", err)
	}

	if !strings.Contains(got, "GT_ROLE=gastown/witness") {
		t.Errorf("expected GT_ROLE=gastown/witness in command, got %q", got)
	}
	if !strings.Contains(got, "BD_ACTOR=gastown/witness") {
		t.Errorf("expected BD_ACTOR=gastown/witness in command, got %q", got)
	}
}

func TestBuildWitnessStartCommand_AgentOverrideWins(t *testing.T) {
	roleConfig := &beads.RoleConfig{
		StartCommand: "exec run --role {role}",
	}

	got, err := buildWitnessStartCommand("/town/rig", "gastown", "/town", "codex", roleConfig)
	if err != nil {
		t.Fatalf("buildWitnessStartCommand: %v", err)
	}
	if strings.Contains(got, "exec run") {
		t.Fatalf("expected agent override to bypass role start_command, got %q", got)
	}
	if !strings.Contains(got, "GT_ROLE=gastown/witness") {
		t.Errorf("expected GT_ROLE=gastown/witness in command, got %q", got)
	}
}

type startupNudgeRecorder struct {
	commands []string
}

func (r *startupNudgeRecorder) NudgeSession(_ string, message string) error {
	r.commands = append(r.commands, message)
	return nil
}

func TestRunWitnessStartupBootstrap_NonHookRuntime(t *testing.T) {
	recorder := &startupNudgeRecorder{}
	runWitnessStartupBootstrap(recorder, "gt-test", &config.RuntimeConfig{
		Hooks: &config.RuntimeHooksConfig{Provider: "none"},
	})

	if len(recorder.commands) != 1 {
		t.Fatalf("expected 1 startup nudge, got %d", len(recorder.commands))
	}
	want := "gt prime && gt mail check --inject"
	if recorder.commands[0] != want {
		t.Fatalf("startup nudge = %q, want %q", recorder.commands[0], want)
	}
}

func TestRunWitnessStartupBootstrap_HookRuntime(t *testing.T) {
	recorder := &startupNudgeRecorder{}
	runWitnessStartupBootstrap(recorder, "gt-test", &config.RuntimeConfig{
		Hooks: &config.RuntimeHooksConfig{Provider: "claude"},
	})

	if len(recorder.commands) != 0 {
		t.Fatalf("expected no startup nudges, got %d", len(recorder.commands))
	}
}
