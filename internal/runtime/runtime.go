// Package runtime provides helpers for runtime-specific integration.
package runtime

import (
	"os"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/claude"
	"github.com/steveyegge/gastown/internal/cli"
	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/copilot"
	"github.com/steveyegge/gastown/internal/gemini"
	"github.com/steveyegge/gastown/internal/opencode"
	"github.com/steveyegge/gastown/internal/templates/commands"
	"github.com/steveyegge/gastown/internal/tmux"
)

func init() {
	// Register hook installers for all agents that support hooks.
	// This replaces the provider switch statement in EnsureSettingsForRole.
	// Adding a new hook-supporting agent = adding a registration here.
	config.RegisterHookInstaller("claude", func(settingsDir, workDir, role, hooksDir, hooksFile string) error {
		return claude.EnsureSettingsForRoleAt(settingsDir, role, hooksDir, hooksFile)
	})
	config.RegisterHookInstaller("gemini", func(settingsDir, workDir, role, hooksDir, hooksFile string) error {
		// Gemini CLI has no --settings flag; install settings in workDir.
		return gemini.EnsureSettingsForRoleAt(workDir, role, hooksDir, hooksFile)
	})
	config.RegisterHookInstaller("opencode", func(settingsDir, workDir, role, hooksDir, hooksFile string) error {
		// OpenCode plugins stay in workDir — no --settings equivalent.
		return opencode.EnsurePluginAt(workDir, hooksDir, hooksFile)
	})
	config.RegisterHookInstaller("copilot", func(settingsDir, workDir, role, hooksDir, hooksFile string) error {
		// Copilot custom instructions stay in workDir — no --settings equivalent.
		return copilot.EnsureSettingsAt(workDir, hooksDir, hooksFile)
	})
}

// EnsureSettingsForRole provisions all agent-specific configuration for a role.
// settingsDir is where provider settings (e.g., .claude/settings.json) are installed.
// workDir is the agent's working directory where slash commands are provisioned.
// For roles like crew/witness/refinery/polecat, settingsDir is a gastown-managed
// parent directory (passed via --settings flag), while workDir is the customer repo.
// For mayor/deacon, settingsDir and workDir are the same.
func EnsureSettingsForRole(settingsDir, workDir, role string, rc *config.RuntimeConfig) error {
	if rc == nil {
		rc = config.DefaultRuntimeConfig()
	}

	if rc.Hooks == nil {
		return nil
	}

	provider := rc.Hooks.Provider
	if provider == "" || provider == "none" {
		return nil
	}

	// 1. Provider-specific settings (settings.json for Claude, plugin for OpenCode, etc.)
	// Hook installers are registered in init() — no switch statement needed.
	if installer := config.GetHookInstaller(provider); installer != nil {
		if err := installer(settingsDir, workDir, role, rc.Hooks.Dir, rc.Hooks.SettingsFile); err != nil {
			return err
		}
	}

	// 2. Slash commands (agent-agnostic, uses shared body with provider-specific frontmatter)
	// Only provision for known agents to maintain backwards compatibility
	if commands.IsKnownAgent(provider) {
		if err := commands.ProvisionFor(workDir, provider); err != nil {
			return err
		}
	}

	return nil
}

// SessionIDFromEnv returns the runtime session ID, if present.
// It checks GT_SESSION_ID_ENV first, then resolves from the current agent's preset,
// and falls back to CLAUDE_SESSION_ID for backwards compatibility.
func SessionIDFromEnv() string {
	if envName := os.Getenv("GT_SESSION_ID_ENV"); envName != "" {
		if sessionID := os.Getenv(envName); sessionID != "" {
			return sessionID
		}
	}
	// Use the current agent's session ID env var from its preset
	if agentName := os.Getenv("GT_AGENT"); agentName != "" {
		if preset := config.GetAgentPresetByName(agentName); preset != nil && preset.SessionIDEnv != "" {
			if sessionID := os.Getenv(preset.SessionIDEnv); sessionID != "" {
				return sessionID
			}
		}
	}
	// Backwards-compatible fallback for sessions without GT_AGENT
	return os.Getenv("CLAUDE_SESSION_ID")
}

// SleepForReadyDelay sleeps for the runtime's configured readiness delay.
func SleepForReadyDelay(rc *config.RuntimeConfig) {
	if rc == nil || rc.Tmux == nil {
		return
	}
	if rc.Tmux.ReadyDelayMs <= 0 {
		return
	}
	time.Sleep(time.Duration(rc.Tmux.ReadyDelayMs) * time.Millisecond)
}

func hasExecutableHooks(rc *config.RuntimeConfig) bool {
	return rc != nil &&
		rc.Hooks != nil &&
		rc.Hooks.Provider != "" &&
		rc.Hooks.Provider != "none" &&
		!rc.Hooks.Informational
}

// StartupFallbackCommands returns commands that approximate Claude hooks when hooks are unavailable.
func StartupFallbackCommands(role string, rc *config.RuntimeConfig) []string {
	if rc == nil {
		rc = config.DefaultRuntimeConfig()
	}
	if hasExecutableHooks(rc) {
		return nil
	}

	role = strings.ToLower(role)
	if role == "boot" {
		// Boot must execute triage immediately on spawn.
		// For non-hook runtimes, do this explicitly instead of waiting for prompt parsing.
		return []string{"gt prime && gt boot triage"}
	}

	command := "gt prime"
	if role == "deacon" {
		// Deacon heartbeat is required for daemon startup health checks.
		// Without this, non-hook runtimes can sit idle at prompt and appear stuck.
		command = "gt deacon heartbeat \"boot patrol\" && " + command
	}
	if isAutonomousRole(role) {
		command += " && gt mail check --inject"
	}
	// NOTE: session-started nudge to deacon removed — it interrupted
	// the deacon's await-signal backoff (exponential sleep). The deacon
	// already wakes on beads activity via bd activity --follow.

	return []string{command}
}

// RunStartupFallback sends the startup fallback commands via tmux.
func RunStartupFallback(t *tmux.Tmux, sessionID, role string, rc *config.RuntimeConfig) error {
	commands := StartupFallbackCommands(role, rc)
	for _, cmd := range commands {
		if err := t.NudgeSession(sessionID, cmd); err != nil {
			return err
		}
	}
	return nil
}

// isAutonomousRole returns true if the given role should automatically
// inject mail check on startup. Autonomous roles (polecat, witness,
// refinery, deacon, boot) operate without human prompting and need mail injection
// to receive work assignments.
//
// Non-autonomous roles (mayor, crew) are human-guided and should not
// have automatic mail injection to avoid confusion.
func isAutonomousRole(role string) bool {
	switch role {
	case "polecat", "witness", "refinery", "deacon", "boot":
		return true
	default:
		return false
	}
}

// DefaultPrimeWaitMs is the default wait time in milliseconds for non-hook agents
// to run gt prime before sending work instructions.
const DefaultPrimeWaitMs = 2000

// StartupFallbackInfo describes what fallback actions are needed for agent startup
// based on the agent's hook and prompt capabilities.
//
// Fallback matrix based on agent capabilities:
//
//	| Hooks | Prompt | Beacon Content           | Context Source      | Work Instructions   |
//	|-------|--------|--------------------------|---------------------|---------------------|
//	| ✓     | ✓      | Standard                 | Hook runs gt prime  | In beacon           |
//	| ✓     | ✗      | Standard (via nudge)     | Hook runs gt prime  | Same nudge          |
//	| ✗     | ✓      | "Run gt prime" (prompt)  | Agent runs manually | Delayed nudge       |
//	| ✗     | ✗      | "Run gt prime" (nudge)   | Agent runs manually | Delayed nudge       |
type StartupFallbackInfo struct {
	// IncludePrimeInBeacon indicates the beacon should include "Run gt prime" instruction.
	// True for non-hook agents where gt prime doesn't run automatically.
	IncludePrimeInBeacon bool

	// SendBeaconNudge indicates the beacon must be sent via nudge (agent has no prompt support).
	// True for agents with PromptMode "none".
	SendBeaconNudge bool

	// SendStartupNudge indicates work instructions need to be sent via nudge.
	// True when beacon doesn't include work instructions (non-hook agents, or hook agents without prompt).
	SendStartupNudge bool

	// StartupNudgeDelayMs is milliseconds to wait before sending work instructions nudge.
	// Allows gt prime to complete for non-hook agents (where it's not automatic).
	StartupNudgeDelayMs int
}

// GetStartupFallbackInfo returns the fallback actions needed based on agent capabilities.
func GetStartupFallbackInfo(rc *config.RuntimeConfig) *StartupFallbackInfo {
	return GetStartupFallbackInfoForRole("", rc)
}

// GetStartupFallbackInfoForRole returns fallback actions needed for startup.
// Role is used to resolve role-aware startup commands for non-hook runtimes.
func GetStartupFallbackInfoForRole(role string, rc *config.RuntimeConfig) *StartupFallbackInfo {
	if rc == nil {
		rc = config.DefaultRuntimeConfig()
	}

	hasHooks := hasExecutableHooks(rc)
	hasPrompt := rc.PromptMode != "none"

	info := &StartupFallbackInfo{}

	if !hasHooks {
		// Non-hook agents need to be told to run gt prime
		info.IncludePrimeInBeacon = true
		info.SendStartupNudge = true
		info.StartupNudgeDelayMs = DefaultPrimeWaitMs

		if !hasPrompt {
			// No prompt support - beacon must be sent via nudge
			info.SendBeaconNudge = true
		}
	} else if !hasPrompt {
		// Has hooks but no prompt - need to nudge beacon + work instructions together
		// Hook runs gt prime synchronously, so no wait needed
		info.SendBeaconNudge = true
		info.SendStartupNudge = true
		info.StartupNudgeDelayMs = 0
	}
	// else: hooks + prompt - nothing needed, all in CLI prompt + hook

	return info
}

// StartupNudgeCommands returns role-aware startup nudge commands for the current runtime capabilities.
func StartupNudgeCommands(role string, rc *config.RuntimeConfig) []string {
	commands := StartupFallbackCommands(role, rc)
	if len(commands) > 0 {
		return commands
	}
	info := GetStartupFallbackInfoForRole(role, rc)
	if info.SendStartupNudge {
		return []string{StartupNudgeContent()}
	}
	return nil
}

// StartupNudgeContent returns the work instructions to send as a startup nudge.
func StartupNudgeContent() string {
	return "Check your hook with `" + cli.Name() + " hook`. If work is present, begin immediately."
}

// BeaconPrimeInstruction returns the instruction to add to beacon for non-hook agents.
func BeaconPrimeInstruction() string {
	return "\n\nRun `" + cli.Name() + " prime` to initialize your context."
}
