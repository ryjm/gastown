package session

import (
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

func TestStartSession_RequiresSessionID(t *testing.T) {
	_, err := StartSession(nil, SessionConfig{
		WorkDir: "/tmp",
		Role:    "polecat",
	})
	if err == nil {
		t.Fatal("expected error for missing SessionID")
	}
	if err.Error() != "SessionID is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStartSession_RequiresWorkDir(t *testing.T) {
	_, err := StartSession(nil, SessionConfig{
		SessionID: "gt-test",
		Role:      "polecat",
	})
	if err == nil {
		t.Fatal("expected error for missing WorkDir")
	}
	if err.Error() != "WorkDir is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStartSession_RequiresRole(t *testing.T) {
	_, err := StartSession(nil, SessionConfig{
		SessionID: "gt-test",
		WorkDir:   "/tmp",
	})
	if err == nil {
		t.Fatal("expected error for missing Role")
	}
	if err.Error() != "Role is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildPrompt_BeaconOnly(t *testing.T) {
	cfg := SessionConfig{
		Beacon: BeaconConfig{
			Recipient: "boot",
			Sender:    "daemon",
			Topic:     "triage",
		},
	}
	prompt := buildPrompt(cfg)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !contains(prompt, "[GAS TOWN]") {
		t.Errorf("prompt should contain beacon: %s", prompt)
	}
}

func TestBuildPrompt_WithInstructions(t *testing.T) {
	cfg := SessionConfig{
		Beacon: BeaconConfig{
			Recipient: "boot",
			Sender:    "daemon",
			Topic:     "triage",
		},
		Instructions: "Run gt boot triage now.",
	}
	prompt := buildPrompt(cfg)
	if !contains(prompt, "Run gt boot triage now.") {
		t.Errorf("prompt should contain instructions: %s", prompt)
	}
	if !contains(prompt, "[GAS TOWN]") {
		t.Errorf("prompt should contain beacon: %s", prompt)
	}
}

func TestBuildCommand_DefaultAgent(t *testing.T) {
	cfg := SessionConfig{
		Role:     "boot",
		TownRoot: "/tmp/town",
	}
	cmd, err := buildCommand(cfg, "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd == "" {
		t.Fatal("expected non-empty command")
	}
}

func TestBuildCommand_WithAgentOverride(t *testing.T) {
	cfg := SessionConfig{
		Role:          "boot",
		TownRoot:      "/tmp/town",
		AgentOverride: "opencode",
	}
	cmd, err := buildCommand(cfg, "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd == "" {
		t.Fatal("expected non-empty command")
	}
}

func TestStartupFallbackPlan_Disabled(t *testing.T) {
	commands, waitBeforeNudge := startupFallbackPlan(SessionConfig{
		Role:               "crew",
		RunStartupFallback: false,
	}, nil)

	if len(commands) != 0 {
		t.Fatalf("expected no fallback commands when disabled, got %v", commands)
	}
	if waitBeforeNudge {
		t.Fatal("expected no wait when fallback is disabled")
	}
}

func TestStartupFallbackPlan_UsesOverrideRole(t *testing.T) {
	rc := &config.RuntimeConfig{
		Provider: "codex",
		Command:  "codex",
		Hooks: &config.RuntimeHooksConfig{
			Provider: "none",
		},
	}

	commands, waitBeforeNudge := startupFallbackPlan(SessionConfig{
		Role:                "crew",
		RunStartupFallback:  true,
		StartupFallbackRole: "polecat",
	}, rc)

	if len(commands) != 1 {
		t.Fatalf("expected one fallback command, got %v", commands)
	}
	if !strings.Contains(commands[0], "gt mail check --inject") {
		t.Fatalf("expected autonomous fallback command to include mail injection, got %q", commands[0])
	}
	if !waitBeforeNudge {
		t.Fatal("expected wait-before-nudge for fallback without ready delay")
	}
}

func TestStartupFallbackPlan_SkipsExtraWaitWhenReadyDelayEnabled(t *testing.T) {
	rc := &config.RuntimeConfig{
		Provider: "codex",
		Command:  "codex",
		Hooks: &config.RuntimeHooksConfig{
			Provider: "none",
		},
	}

	commands, waitBeforeNudge := startupFallbackPlan(SessionConfig{
		Role:               "crew",
		RunStartupFallback: true,
		ReadyDelay:         true,
	}, rc)

	if len(commands) == 0 {
		t.Fatal("expected fallback commands for non-hook crew runtime")
	}
	if waitBeforeNudge {
		t.Fatal("expected no extra wait when ready delay already ran")
	}
}

func TestStartupFallbackPlan_HookRuntimeHasNoFallback(t *testing.T) {
	rc := &config.RuntimeConfig{
		Provider: "claude",
		Command:  "claude",
		Hooks: &config.RuntimeHooksConfig{
			Provider:      "claude",
			Informational: false,
		},
	}

	commands, waitBeforeNudge := startupFallbackPlan(SessionConfig{
		Role:               "crew",
		RunStartupFallback: true,
	}, rc)

	if len(commands) != 0 {
		t.Fatalf("expected no fallback commands for hook-capable runtime, got %v", commands)
	}
	if waitBeforeNudge {
		t.Fatal("expected no wait when there are no fallback commands")
	}
}

func TestKillExistingSession_NoSession(t *testing.T) {
	// KillExistingSession with nil tmux would panic, but we test the logic
	// by verifying it's callable. Full integration tests need a real tmux.
	// This test verifies the function signature and basic flow.
	t.Skip("requires tmux for integration testing")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
