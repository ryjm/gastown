package daemon

import (
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

func TestDeaconStaleNudgeCommand_NonHookRuntimeUsesDeterministicCommand(t *testing.T) {
	rc := &config.RuntimeConfig{
		Provider: "codex",
		Command:  "codex",
		Hooks: &config.RuntimeHooksConfig{
			Provider: "none",
		},
	}

	got := deaconStaleNudgeCommand(rc)

	for _, want := range []string{
		"gt deacon heartbeat",
		"heartbeat stale poke",
		"gt prime",
		"gt mail check --inject",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("deaconStaleNudgeCommand() = %q, expected to contain %q", got, want)
		}
	}
}

func TestDeaconStaleNudgeCommand_HookRuntimeUsesHealthMessage(t *testing.T) {
	rc := &config.RuntimeConfig{
		Provider: "claude",
		Command:  "claude",
		Hooks: &config.RuntimeHooksConfig{
			Provider:      "claude",
			Informational: false,
		},
	}

	got := deaconStaleNudgeCommand(rc)
	if got != defaultDeaconStaleNudge {
		t.Fatalf("deaconStaleNudgeCommand() = %q, want %q", got, defaultDeaconStaleNudge)
	}
}

func TestDeaconStaleNudgeCommand_DefaultsToHealthMessage(t *testing.T) {
	if got := deaconStaleNudgeCommand(nil); got != defaultDeaconStaleNudge {
		t.Fatalf("deaconStaleNudgeCommand(nil) = %q, want %q", got, defaultDeaconStaleNudge)
	}
}
