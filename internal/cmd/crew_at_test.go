package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

func setupCrewAtCommandConfig(t *testing.T) (string, string) {
	t.Helper()

	townRoot := t.TempDir()
	rigPath := filepath.Join(townRoot, "gastown")
	if err := os.MkdirAll(rigPath, 0755); err != nil {
		t.Fatalf("mkdir rig path: %v", err)
	}

	if err := config.SaveTownSettings(config.TownSettingsPath(townRoot), config.NewTownSettings()); err != nil {
		t.Fatalf("SaveTownSettings: %v", err)
	}
	if err := config.SaveRigSettings(config.RigSettingsPath(rigPath), config.NewRigSettings()); err != nil {
		t.Fatalf("SaveRigSettings: %v", err)
	}

	return townRoot, rigPath
}

func TestCrewAtStartupBeacon_DefaultTopic(t *testing.T) {
	t.Parallel()

	beacon := crewAtStartupBeacon("gastown", "toast", "")
	if !strings.Contains(beacon, "[GAS TOWN] gastown/crew/toast <- human") {
		t.Fatalf("beacon missing startup identity: %q", beacon)
	}
	if !strings.Contains(beacon, " start") {
		t.Fatalf("beacon should default topic to start: %q", beacon)
	}
}

func TestBuildCrewAtStartupCommand_TopicParity(t *testing.T) {
	t.Parallel()

	_, rigPath := setupCrewAtCommandConfig(t)

	startCmd, err := buildCrewAtStartupCommand("gastown", "toast", rigPath, "start", "gemini", nil, "")
	if err != nil {
		t.Fatalf("buildCrewAtStartupCommand(start): %v", err)
	}
	restartCmd, err := buildCrewAtStartupCommand("gastown", "toast", rigPath, "restart", "gemini", nil, "")
	if err != nil {
		t.Fatalf("buildCrewAtStartupCommand(restart): %v", err)
	}

	for _, cmd := range []string{startCmd, restartCmd} {
		if !strings.Contains(cmd, "GT_ROLE=gastown/crew/toast") {
			t.Fatalf("startup command missing GT_ROLE: %q", cmd)
		}
		if !strings.Contains(cmd, "gemini --approval-mode yolo") {
			t.Fatalf("startup command missing override runtime: %q", cmd)
		}
	}

	if !strings.Contains(startCmd, "[GAS TOWN] gastown/crew/toast <- human") || !strings.Contains(startCmd, " start") {
		t.Fatalf("start command missing start beacon: %q", startCmd)
	}
	if !strings.Contains(restartCmd, "[GAS TOWN] gastown/crew/toast <- human") || !strings.Contains(restartCmd, " restart") {
		t.Fatalf("restart command missing restart beacon: %q", restartCmd)
	}
}

func TestBuildCrewAtStartupCommand_PrependsConfigDirEnv(t *testing.T) {
	t.Parallel()

	_, rigPath := setupCrewAtCommandConfig(t)

	rc := &config.RuntimeConfig{
		Session: &config.RuntimeSessionConfig{
			ConfigDirEnv: "CLAUDE_CONFIG_DIR",
		},
	}

	cmd, err := buildCrewAtStartupCommand("gastown", "toast", rigPath, "start", "gemini", rc, "/tmp/account")
	if err != nil {
		t.Fatalf("buildCrewAtStartupCommand: %v", err)
	}
	if !strings.Contains(cmd, "CLAUDE_CONFIG_DIR=") || !strings.Contains(cmd, "/tmp/account") {
		t.Fatalf("startup command missing config dir env prepend: %q", cmd)
	}
}

func TestBuildCrewAtStartupCommand_InvalidOverride(t *testing.T) {
	t.Parallel()

	_, rigPath := setupCrewAtCommandConfig(t)

	_, err := buildCrewAtStartupCommand("gastown", "toast", rigPath, "start", "not-a-real-agent", nil, "")
	if err == nil {
		t.Fatal("expected error for invalid agent override")
	}
}
