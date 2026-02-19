package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/runtime"
	"github.com/steveyegge/gastown/internal/session"
	"github.com/steveyegge/gastown/internal/tmux"
)

var (
	startupFallbackCommands = runtime.StartupFallbackCommands
	startupFallbackSleep    = runtime.SleepForReadyDelay
	startupFallbackRun      = runtime.RunStartupFallback
	tmuxRunShellBackground  = func(script string) error {
		return exec.Command("tmux", "run-shell", "-b", script).Run()
	}
)

func roleForSessionStartupBootstrap(identity *session.AgentIdentity) string {
	if identity == nil {
		return ""
	}
	if identity.Role == session.RoleDeacon && identity.Name == "boot" {
		return "boot"
	}
	return string(identity.Role)
}

func runtimeConfigForSessionStartupBootstrap(sessionName, townRoot string) (string, *config.RuntimeConfig, error) {
	identity, err := session.ParseSessionName(sessionName)
	if err != nil {
		return "", nil, fmt.Errorf("parsing session name %q: %w", sessionName, err)
	}

	role := roleForSessionStartupBootstrap(identity)
	rigPath := ""
	if identity.Rig != "" && townRoot != "" {
		rigPath = filepath.Join(townRoot, identity.Rig)
	}

	return role, config.ResolveRoleAgentConfig(role, townRoot, rigPath), nil
}

func runRespawnStartupBootstrap(t *tmux.Tmux, sessionID, role string, runtimeConfig *config.RuntimeConfig) error {
	commands := startupFallbackCommands(role, runtimeConfig)
	if len(commands) == 0 {
		return nil
	}

	startupFallbackSleep(runtimeConfig)
	return startupFallbackRun(t, sessionID, role, runtimeConfig)
}

func scheduleRespawnStartupBootstrap(sessionID, role string, runtimeConfig *config.RuntimeConfig) error {
	commands := startupFallbackCommands(role, runtimeConfig)
	if len(commands) == 0 {
		return nil
	}

	delay := startupBootstrapDelay(runtimeConfig)
	for _, command := range commands {
		script := buildDeferredNudgeScript(sessionID, command, delay)
		if err := tmuxRunShellBackground(script); err != nil {
			return fmt.Errorf("scheduling startup bootstrap for %s: %w", sessionID, err)
		}
		// Only delay the first message.
		delay = 0
	}
	return nil
}

func startupBootstrapDelay(runtimeConfig *config.RuntimeConfig) time.Duration {
	if runtimeConfig == nil || runtimeConfig.Tmux == nil || runtimeConfig.Tmux.ReadyDelayMs <= 0 {
		return 0
	}
	return time.Duration(runtimeConfig.Tmux.ReadyDelayMs) * time.Millisecond
}

func buildDeferredNudgeScript(sessionID, command string, delay time.Duration) string {
	steps := make([]string, 0, 3)
	if delay > 0 {
		steps = append(steps, fmt.Sprintf("sleep %.3f", delay.Seconds()))
	}

	quotedSession := config.ShellQuote(sessionID)
	quotedCommand := config.ShellQuote(command)
	steps = append(steps,
		fmt.Sprintf("tmux send-keys -t %s -l %s", quotedSession, quotedCommand),
		fmt.Sprintf("tmux send-keys -t %s Enter", quotedSession),
	)
	return strings.Join(steps, " && ")
}
