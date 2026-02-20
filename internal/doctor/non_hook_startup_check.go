package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/runtime"
)

// NonHookStartupParityCheck verifies startup fallback wiring for non-hook runtimes.
// This catches misconfigurations where Codex-like agents can start without
// receiving prime/hook instructions and then idle at prompt.
type NonHookStartupParityCheck struct {
	BaseCheck
}

type startupRoleTarget struct {
	role              string
	scope             string
	rigPath           string
	requireMailCheck  bool
	requireBootTriage bool
}

// NewNonHookStartupParityCheck creates a new non-hook startup parity check.
func NewNonHookStartupParityCheck() *NonHookStartupParityCheck {
	return &NonHookStartupParityCheck{
		BaseCheck: BaseCheck{
			CheckName:        "non-hook-startup-parity",
			CheckDescription: "Verify non-hook runtimes have startup bootstrap parity",
			CheckCategory:    CategoryConfig,
		},
	}
}

// Run validates startup fallback prerequisites for all non-hook role targets.
func (c *NonHookStartupParityCheck) Run(ctx *CheckContext) *CheckResult {
	if ctx == nil || ctx.TownRoot == "" {
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusOK,
			Message:  "No town root provided (skipped)",
			Category: c.Category(),
		}
	}

	targets := c.targets(ctx)
	issues := make([]string, 0)
	nonHookChecked := 0

	for _, target := range targets {
		rc := config.ResolveRoleAgentConfig(target.role, ctx.TownRoot, target.rigPath)
		targetIssues, checked := c.validateTarget(target, rc)
		issues = append(issues, targetIssues...)
		if checked {
			nonHookChecked++
		}
	}

	if len(issues) == 0 {
		msg := "No non-hook startup roles configured"
		if nonHookChecked > 0 {
			msg = fmt.Sprintf("Validated non-hook startup parity for %d role target(s)", nonHookChecked)
		}
		return &CheckResult{
			Name:     c.Name(),
			Status:   StatusOK,
			Message:  msg,
			Category: c.Category(),
		}
	}

	return &CheckResult{
		Name:     c.Name(),
		Status:   StatusError,
		Message:  fmt.Sprintf("Found %d non-hook startup parity issue(s)", len(issues)),
		Details:  issues,
		FixHint:  "For codex/non-hook runtimes: set hooks.provider to 'none' (or informational=true for instruction-only hooks), then ensure startup fallback includes 'gt prime' and 'gt mail check --inject' for autonomous roles.",
		Category: c.Category(),
	}
}

func (c *NonHookStartupParityCheck) targets(ctx *CheckContext) []startupRoleTarget {
	targets := []startupRoleTarget{
		{role: "deacon", scope: "town/deacon", requireMailCheck: true},
		{role: "boot", scope: "town/boot", requireBootTriage: true},
	}

	for _, rigPath := range c.rigPaths(ctx) {
		rigName := filepath.Base(rigPath)
		targets = append(targets,
			startupRoleTarget{role: "polecat", scope: rigName + "/polecat", rigPath: rigPath, requireMailCheck: true},
			startupRoleTarget{role: "witness", scope: rigName + "/witness", rigPath: rigPath, requireMailCheck: true},
			startupRoleTarget{role: "refinery", scope: rigName + "/refinery", rigPath: rigPath, requireMailCheck: true},
			startupRoleTarget{role: "crew", scope: rigName + "/crew", rigPath: rigPath},
		)
	}

	return targets
}

func (c *NonHookStartupParityCheck) rigPaths(ctx *CheckContext) []string {
	if ctx.RigName != "" {
		rigPath := ctx.RigPath()
		if info, err := os.Stat(rigPath); err == nil && info.IsDir() {
			return []string{rigPath}
		}
		return nil
	}
	return findAllRigs(ctx.TownRoot)
}

func (c *NonHookStartupParityCheck) validateTarget(target startupRoleTarget, rc *config.RuntimeConfig) ([]string, bool) {
	if rc == nil {
		return []string{fmt.Sprintf("%s: unable to resolve runtime config", target.scope)}, false
	}

	hooksEnabled := hasExecutableHooks(rc)

	// Known non-hook providers must not be configured as executable-hook runtimes.
	if isKnownNonHookProvider(rc.Provider) && hooksEnabled {
		hooksProvider := "none"
		if rc.Hooks != nil && rc.Hooks.Provider != "" {
			hooksProvider = rc.Hooks.Provider
		}
		return []string{fmt.Sprintf("%s: provider %q is non-hook but hooks.provider=%q is executable", target.scope, rc.Provider, hooksProvider)}, false
	}

	// Hook-capable runtime is coherent; fallback parity checks are unnecessary.
	if hooksEnabled {
		return nil, false
	}

	issues := make([]string, 0)

	info := runtime.GetStartupFallbackInfo(rc)
	if !info.IncludePrimeInBeacon {
		issues = append(issues, fmt.Sprintf("%s: fallback must include prime instruction in beacon", target.scope))
	}
	if !info.SendStartupNudge {
		issues = append(issues, fmt.Sprintf("%s: fallback must send startup nudge for non-hook runtime", target.scope))
	}
	if info.StartupNudgeDelayMs <= 0 {
		issues = append(issues, fmt.Sprintf("%s: fallback startup nudge delay must be > 0 for non-hook runtime", target.scope))
	}
	if strings.EqualFold(rc.PromptMode, "none") && !info.SendBeaconNudge {
		issues = append(issues, fmt.Sprintf("%s: prompt-less non-hook runtime must send beacon via nudge", target.scope))
	}

	commands := runtime.StartupFallbackCommands(target.role, rc)
	if len(commands) == 0 {
		issues = append(issues, fmt.Sprintf("%s: fallback commands missing for non-hook runtime", target.scope))
		return issues, true
	}

	joined := strings.Join(commands, " && ")
	if !strings.Contains(joined, "gt prime") {
		issues = append(issues, fmt.Sprintf("%s: fallback commands must include 'gt prime'", target.scope))
	}
	if target.requireMailCheck && !strings.Contains(joined, "gt mail check --inject") {
		issues = append(issues, fmt.Sprintf("%s: fallback commands must include 'gt mail check --inject'", target.scope))
	}
	if target.requireBootTriage && !strings.Contains(joined, "gt boot triage") {
		issues = append(issues, fmt.Sprintf("%s: fallback commands must include 'gt boot triage'", target.scope))
	}

	return issues, true
}

func hasExecutableHooks(rc *config.RuntimeConfig) bool {
	if rc == nil || rc.Hooks == nil {
		return false
	}
	provider := strings.TrimSpace(strings.ToLower(rc.Hooks.Provider))
	if provider == "" || provider == "none" {
		return false
	}
	return !rc.Hooks.Informational
}

func isKnownNonHookProvider(provider string) bool {
	if provider == "" {
		return false
	}
	preset := config.GetAgentPresetByName(provider)
	return preset != nil && !preset.SupportsHooks
}
