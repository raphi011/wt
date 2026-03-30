package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
)

// hookParams holds everything needed to run hooks around a command.
type hookParams struct {
	HooksCfg  config.HooksConfig
	ConfigDir string // ~/.wt/ config dir
	WtPath    string // worktree path (used as workDir for hook execution)
	RepoPath  string
	RepoName  string
	Branch    string
	Trigger   hooks.CommandType
	Action    string
	HookNames []string
	NoHook    bool
	Env       map[string]string
}

// withHooks runs before-hooks, then fn, then after-hooks.
// If before-hooks fail, fn is not called and the error is returned.
// After-hook failures are logged as warnings.
func withHooks(ctx context.Context, p hookParams, fn func() error) error {
	hookCtx := hooks.Context{
		WorktreeDir: p.WtPath,
		RepoDir:     p.RepoPath,
		Branch:      p.Branch,
		Repo:        p.RepoName,
		Trigger:     string(p.Trigger),
		Action:      p.Action,
		ConfigDir:   p.ConfigDir,
		Env:         p.Env,
	}

	// Before hooks (can abort)
	beforeMatches, err := hooks.SelectHooks(p.HooksCfg, p.HookNames, p.NoHook, p.Trigger, p.Action, hooks.PhaseBefore)
	if err != nil {
		return err
	}
	hookCtx.Phase = hooks.PhaseBefore
	if err := hooks.RunBeforeHooks(ctx, beforeMatches, hookCtx, p.WtPath); err != nil {
		return fmt.Errorf("before-hook aborted %s: %w", p.Trigger, err)
	}

	// Core logic
	if err := fn(); err != nil {
		return err
	}

	// After hooks (non-fatal)
	afterMatches, err := hooks.SelectHooks(p.HooksCfg, p.HookNames, p.NoHook, p.Trigger, p.Action, hooks.PhaseAfter)
	if err != nil {
		return err
	}
	if len(afterMatches) > 0 {
		hookCtx.Phase = hooks.PhaseAfter
		hooks.RunForEach(ctx, afterMatches, hookCtx, p.WtPath)
	}

	return nil
}

// buildHookParams creates a hookParams from config and raw env strings.
// Returns error if env parsing or config dir resolution fails.
func buildHookParams(cfg *config.Config, wtPath, repoPath, repoName, branch string, trigger hooks.CommandType, action string, hookNames []string, noHook bool, env []string) (hookParams, error) {
	hookEnv, err := hooks.ParseEnvWithStdin(env)
	if err != nil {
		return hookParams{}, err
	}

	configDir, err := cfg.GetWtDir()
	if err != nil {
		return hookParams{}, fmt.Errorf("config dir: %w", err)
	}

	return hookParams{
		HooksCfg:  cfg.Hooks,
		ConfigDir: configDir,
		WtPath:    wtPath,
		RepoPath:  repoPath,
		RepoName:  repoName,
		Branch:    branch,
		Trigger:   trigger,
		Action:    action,
		HookNames: hookNames,
		NoHook:    noHook,
		Env:       hookEnv,
	}, nil
}

// recordHistory records a worktree access to the history file.
// Errors are logged as warnings, not returned.
func recordHistory(ctx context.Context, cfg *config.Config, wtPath, repoName, branch string) {
	l := log.FromContext(ctx)
	histPath, err := cfg.GetHistoryPath()
	if err != nil {
		l.Printf("Warning: failed to determine history path: %v\n", err)
		return
	}
	if err := history.RecordAccess(wtPath, repoName, branch, histPath); err != nil {
		l.Printf("Warning: failed to record history: %v\n", err)
	}
}

// registerHookFlags adds the standard --hook, --no-hook, and --arg flags to a command.
func registerHookFlags(cmd *cobra.Command, hookNames *[]string, noHook *bool, env *[]string) {
	cmd.Flags().StringSliceVar(hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(noHook, "no-hook", false, "Skip hooks")
	cmd.Flags().StringSliceVarP(env, "arg", "a", nil, "Set hook variable (KEY=VALUE or KEY for boolean)")
	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)
	cmd.RegisterFlagCompletionFunc("arg", cobra.NoFileCompletions)
}
