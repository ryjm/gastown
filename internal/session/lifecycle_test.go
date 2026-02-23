package session

import (
	"testing"

	"github.com/steveyegge/gastown/internal/runtime"
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

func TestKillExistingSession_NoSession(t *testing.T) {
	// KillExistingSession with nil tmux would panic, but we test the logic
	// by verifying it's callable. Full integration tests need a real tmux.
	// This test verifies the function signature and basic flow.
	t.Skip("requires tmux for integration testing")
}

func TestStartupFallbackRole(t *testing.T) {
	cfg := SessionConfig{Role: "boot", StartupFallbackRole: "deacon"}
	if got := startupFallbackRole(cfg); got != "deacon" {
		t.Fatalf("startupFallbackRole() = %q, want deacon", got)
	}

	cfg.StartupFallbackRole = ""
	if got := startupFallbackRole(cfg); got != "boot" {
		t.Fatalf("startupFallbackRole() = %q, want boot", got)
	}
}

func TestApplyStartupFallbackToBeacon(t *testing.T) {
	beacon := BeaconConfig{}
	info := &runtime.StartupFallbackInfo{
		IncludePrimeInBeacon: true,
		SendStartupNudge:     true,
	}

	got := applyStartupFallbackToBeacon(beacon, info)
	if !got.IncludePrimeInstruction {
		t.Fatal("expected IncludePrimeInstruction to be enabled")
	}
	if !got.ExcludeWorkInstructions {
		t.Fatal("expected ExcludeWorkInstructions to be enabled")
	}
}

func TestBuildStartupNudgeSequence_CombinedPromptless(t *testing.T) {
	info := &runtime.StartupFallbackInfo{
		SendBeaconNudge:     true,
		SendStartupNudge:    true,
		StartupNudgeDelayMs: 0,
	}

	sequence := buildStartupNudgeSequence("beacon", info, []string{"cmd"})
	if !sequence.CombineNudges {
		t.Fatal("expected combined nudge sequence")
	}
	if !sequence.SendBeacon {
		t.Fatal("expected beacon nudge to be enabled")
	}
	if len(sequence.StartupCommands) != 1 || sequence.StartupCommands[0] != "cmd" {
		t.Fatalf("unexpected startup commands: %v", sequence.StartupCommands)
	}
}

func TestBuildStartupNudgeSequence_DelayedSeparate(t *testing.T) {
	info := &runtime.StartupFallbackInfo{
		SendBeaconNudge:     true,
		SendStartupNudge:    true,
		StartupNudgeDelayMs: 1500,
	}

	sequence := buildStartupNudgeSequence("beacon", info, []string{"cmd"})
	if sequence.CombineNudges {
		t.Fatal("expected separate nudges when delay is set")
	}
	if sequence.StartupDelayMs != 1500 {
		t.Fatalf("StartupDelayMs = %d, want 1500", sequence.StartupDelayMs)
	}
}

func TestBuildStartupNudgeSequence_FallsBackToInstructionContent(t *testing.T) {
	info := &runtime.StartupFallbackInfo{
		SendStartupNudge: true,
	}

	sequence := buildStartupNudgeSequence("", info, nil)
	if len(sequence.StartupCommands) != 1 {
		t.Fatalf("expected single fallback startup command, got %d", len(sequence.StartupCommands))
	}
	if sequence.StartupCommands[0] != runtime.StartupNudgeContent() {
		t.Fatalf("fallback startup command = %q, want %q", sequence.StartupCommands[0], runtime.StartupNudgeContent())
	}
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
