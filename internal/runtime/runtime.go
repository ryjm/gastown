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

type startupCapabilities struct {
	HasHooks  bool
	HasPrompt bool
}

func resolveStartupCapabilities(rc *config.RuntimeConfig) startupCapabilities {
	if rc == nil {
		rc = config.DefaultRuntimeConfig()
	}
	return startupCapabilities{
		HasHooks:  rc.Hooks != nil && rc.Hooks.Provider != "" && rc.Hooks.Provider != "none" && !rc.Hooks.Informational,
		HasPrompt: rc.PromptMode != "none",
	}
}

// StartupFallbackCommands returns commands that approximate Claude hooks when hooks are unavailable.
func StartupFallbackCommands(role string, rc *config.RuntimeConfig) []string {
	capabilities := resolveStartupCapabilities(rc)
	if capabilities.HasHooks {
		return nil
	}

	rolePlan := config.StartupFallbackPlanForRole(role)
	commandParts := make([]string, 0, 4)

	if rolePlan.PrePrimeCommand != "" {
		commandParts = append(commandParts, rolePlan.PrePrimeCommand)
	}

	primeCommand := strings.TrimSpace(rolePlan.PrimeCommand)
	if primeCommand == "" {
		primeCommand = "gt prime"
	}
	commandParts = append(commandParts, primeCommand)

	if rolePlan.AutoMailInject {
		commandParts = append(commandParts, "gt mail check --inject")
	}

	if !capabilities.HasPrompt && rolePlan.PromptlessCommand != "" {
		commandParts = append(commandParts, rolePlan.PromptlessCommand)
	}

	// NOTE: session-started nudge to deacon removed — it interrupted
	// the deacon's await-signal backoff (exponential sleep). The deacon
	// already wakes on beads activity via bd activity --follow.

	return []string{strings.Join(commandParts, " && ")}
}

// RunStartupFallback sends the startup fallback commands via tmux.
func RunStartupFallback(t *tmux.Tmux, sessionID, role string, rc *config.RuntimeConfig) error {
	// Legacy wrapper for callers that only need fallback commands.
	// Preserve previous behavior: immediate dispatch with no extra ready-delay wait.
	contract := BuildStartupBootstrapContract(StartupBootstrapSpec{
		Role:                    role,
		IncludeFallbackCommands: true,
		ReadyDelayApplied:       true,
	}, rc)
	return ExecuteStartupBootstrapContract(t, sessionID, contract)
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

// StartupBootstrapSpec describes which startup bootstrap actions to plan.
// The spec allows each session entrypoint to request the same capability-aware
// behavior (beacon nudge, startup nudge, fallback commands) without re-implementing
// sequencing logic.
type StartupBootstrapSpec struct {
	// Role determines which fallback commands are generated.
	Role string

	// BeaconMessage is the startup beacon text. If empty, no beacon nudge is planned.
	BeaconMessage string

	// StartupNudgeMessage is the startup instruction nudge text.
	// If empty, no startup nudge is planned.
	StartupNudgeMessage string

	// IncludeFallbackCommands appends legacy fallback command nudges.
	IncludeFallbackCommands bool

	// ReadyDelayApplied indicates whether caller already ran SleepForReadyDelay.
	// When false and fallback commands are planned, the contract inserts the
	// runtime ready-delay wait before dispatching fallback commands.
	ReadyDelayApplied bool
}

// StartupBootstrapStepKind identifies one bootstrap action in execution order.
type StartupBootstrapStepKind string

const (
	// StartupBootstrapStepWait waits before dispatching later bootstrap steps.
	StartupBootstrapStepWait StartupBootstrapStepKind = "wait"

	// StartupBootstrapStepNudge sends a tmux nudge command/message.
	StartupBootstrapStepNudge StartupBootstrapStepKind = "nudge"
)

// StartupBootstrapStep is one ordered bootstrap action.
type StartupBootstrapStep struct {
	Kind StartupBootstrapStepKind

	// Delay applies only when Kind == StartupBootstrapStepWait.
	Delay time.Duration

	// Command applies only when Kind == StartupBootstrapStepNudge.
	Command string
}

// StartupBootstrapContract is the shared startup bootstrap execution plan.
// It exposes computed capability info and explicit ordered steps that
// session entrypoints can execute consistently.
type StartupBootstrapContract struct {
	Info  *StartupFallbackInfo
	Steps []StartupBootstrapStep
}

type startupBootstrapNudger interface {
	NudgeSession(sessionID, message string) error
}

// BuildStartupBootstrapContract creates the ordered startup bootstrap plan.
// This is the canonical startup contract for capability-aware startup behavior.
func BuildStartupBootstrapContract(spec StartupBootstrapSpec, rc *config.RuntimeConfig) *StartupBootstrapContract {
	info := GetStartupFallbackInfo(rc)
	steps := make([]StartupBootstrapStep, 0, 6)

	if info.SendBeaconNudge && info.SendStartupNudge && info.StartupNudgeDelayMs == 0 && spec.BeaconMessage != "" && spec.StartupNudgeMessage != "" {
		// Hook-capable but prompt-less runtimes can receive a single combined
		// message because hooks already handled gt prime synchronously.
		steps = append(steps, StartupBootstrapStep{
			Kind:    StartupBootstrapStepNudge,
			Command: spec.BeaconMessage + "\n\n" + spec.StartupNudgeMessage,
		})
	} else {
		if info.SendBeaconNudge && spec.BeaconMessage != "" {
			steps = append(steps, StartupBootstrapStep{
				Kind:    StartupBootstrapStepNudge,
				Command: spec.BeaconMessage,
			})
		}

		if info.SendStartupNudge && spec.StartupNudgeMessage != "" {
			if info.StartupNudgeDelayMs > 0 {
				steps = append(steps, StartupBootstrapStep{
					Kind:  StartupBootstrapStepWait,
					Delay: time.Duration(info.StartupNudgeDelayMs) * time.Millisecond,
				})
			}
			steps = append(steps, StartupBootstrapStep{
				Kind:    StartupBootstrapStepNudge,
				Command: spec.StartupNudgeMessage,
			})
		}
	}

	if spec.IncludeFallbackCommands {
		commands := StartupFallbackCommands(spec.Role, rc)
		if len(commands) > 0 {
			if !spec.ReadyDelayApplied {
				if readyDelay := readyDelayDuration(rc); readyDelay > 0 {
					steps = append(steps, StartupBootstrapStep{
						Kind:  StartupBootstrapStepWait,
						Delay: readyDelay,
					})
				}
			}
			for _, command := range commands {
				steps = append(steps, StartupBootstrapStep{
					Kind:    StartupBootstrapStepNudge,
					Command: command,
				})
			}
		}
	}

	return &StartupBootstrapContract{
		Info:  info,
		Steps: steps,
	}
}

// ExecuteStartupBootstrapContract runs the ordered startup bootstrap steps.
func ExecuteStartupBootstrapContract(t *tmux.Tmux, sessionID string, contract *StartupBootstrapContract) error {
	return executeStartupBootstrapContract(t, sessionID, contract, time.Sleep)
}

func executeStartupBootstrapContract(t startupBootstrapNudger, sessionID string, contract *StartupBootstrapContract, sleepFn func(time.Duration)) error {
	if contract == nil {
		return nil
	}
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	for _, step := range contract.Steps {
		switch step.Kind {
		case StartupBootstrapStepWait:
			if step.Delay > 0 {
				sleepFn(step.Delay)
			}
		case StartupBootstrapStepNudge:
			if step.Command == "" {
				continue
			}
			if err := t.NudgeSession(sessionID, step.Command); err != nil {
				return err
			}
		}
	}

	return nil
}

func readyDelayDuration(rc *config.RuntimeConfig) time.Duration {
	if rc == nil {
		rc = config.DefaultRuntimeConfig()
	}
	if rc.Tmux == nil || rc.Tmux.ReadyDelayMs <= 0 {
		return 0
	}
	return time.Duration(rc.Tmux.ReadyDelayMs) * time.Millisecond
}

// GetStartupFallbackInfo returns the fallback actions needed based on agent capabilities.
func GetStartupFallbackInfo(rc *config.RuntimeConfig) *StartupFallbackInfo {
	capabilities := resolveStartupCapabilities(rc)

	info := &StartupFallbackInfo{}

	if !capabilities.HasHooks {
		// Non-hook agents need to be told to run gt prime
		info.IncludePrimeInBeacon = true
		info.SendStartupNudge = true
		info.StartupNudgeDelayMs = DefaultPrimeWaitMs

		if !capabilities.HasPrompt {
			// No prompt support - beacon must be sent via nudge
			info.SendBeaconNudge = true
		}
	} else if !capabilities.HasPrompt {
		// Has hooks but no prompt - need to nudge beacon + work instructions together
		// Hook runs gt prime synchronously, so no wait needed
		info.SendBeaconNudge = true
		info.SendStartupNudge = true
		info.StartupNudgeDelayMs = 0
	}
	// else: hooks + prompt - nothing needed, all in CLI prompt + hook

	return info
}

// StartupNudgeContent returns the work instructions to send as a startup nudge.
func StartupNudgeContent() string {
	return "Check your hook with `" + cli.Name() + " hook`. If work is present, begin immediately."
}

// BeaconPrimeInstruction returns the instruction to add to beacon for non-hook agents.
func BeaconPrimeInstruction() string {
	return "\n\nRun `" + cli.Name() + " prime` to initialize your context."
}
