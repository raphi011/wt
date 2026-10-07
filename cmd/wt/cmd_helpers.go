package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

// hookFlags holds the raw CLI-level hook configuration flags.
// This triplet is passed through many functions unchanged before being
// hydrated into a hooks.Operation by hookOperation.
type hookFlags struct {
	HookNames []string          // --hook flag values
	NoHook    bool              // --no-hook flag
	RawArgs   []string          // --arg flag values (raw KEY=VALUE strings, not yet parsed)
	Env       map[string]string // parsed --arg values, set by parseArgs
}

// parseArgs parses RawArgs into Env, reading stdin for KEY=- values.
// Must be called once per command invocation before hookOperation:
// stdin is drained by the first read.
func (hf *hookFlags) parseArgs() error {
	env, err := hooks.ParseEnvWithStdin(hf.RawArgs)
	if err != nil {
		return err
	}
	hf.Env = env
	return nil
}

// hookOperation creates the hooks.Operation of a command acting on a worktree.
// hf.parseArgs must have been called. Returns error if config dir resolution fails.
func hookOperation(cfg *config.Config, repo registry.Repo, wtPath, branch string, trigger hooks.CommandType, action string, hf hookFlags) (hooks.Operation, error) {
	configDir, err := cfg.GetWtDir()
	if err != nil {
		return hooks.Operation{}, fmt.Errorf("config dir: %w", err)
	}

	return hooks.Operation{
		Hooks:       cfg.Hooks,
		ConfigDir:   configDir,
		Trigger:     trigger,
		Action:      action,
		RepoDir:     repo.Path,
		Repo:        repo.Name,
		WorktreeDir: wtPath,
		Branch:      branch,
		HookNames:   hf.HookNames,
		NoHook:      hf.NoHook,
		Env:         hf.Env,
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
func registerHookFlags(cmd *cobra.Command, hf *hookFlags) {
	cmd.Flags().StringSliceVar(&hf.HookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&hf.NoHook, "no-hook", false, "Skip hooks")
	cmd.Flags().StringSliceVarP(&hf.RawArgs, "arg", "a", nil, "Set hook variable (KEY=VALUE or KEY for boolean)")
	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("hook", completeHooks))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("arg", cobra.NoFileCompletions))
}

// errMultipleRepoMatches is returned when multiple registered repos have
// remotes matching the same org/repo path.
var errMultipleRepoMatches = errors.New("multiple registered repos match")

// findRepoByRemoteRepoPath searches all registered repos for one whose remote
// URLs match the given org/repo path (e.g. "n26/de.tech26.protectedaccounts").
// Checks all remotes (origin, upstream, etc.), not just origin.
// Comparison is case-insensitive.
func findRepoByRemoteRepoPath(ctx context.Context, reg *registry.Registry, orgRepo string) (registry.Repo, error) {
	l := log.FromContext(ctx)
	var matches []registry.Repo
	var skipped []string

	for _, repo := range reg.Repos {
		if ctx.Err() != nil {
			return registry.Repo{}, ctx.Err()
		}
		remoteURLs, err := git.GetRemoteURLs(ctx, repo.Path)
		if err != nil {
			l.Debug("skipping repo for remote match", "repo", repo.Name, "path", repo.Path, "error", err)
			skipped = append(skipped, repo.Name)
			continue
		}
		for _, url := range remoteURLs {
			if strings.EqualFold(forge.ExtractRepoPath(url), orgRepo) {
				matches = append(matches, repo)
				break
			}
		}
	}

	switch len(matches) {
	case 0:
		msg := fmt.Sprintf("no registered repo found with remote matching %s", orgRepo)
		if len(skipped) > 0 {
			msg += fmt.Sprintf(" (could not check %d repo(s): %s)", len(skipped), strings.Join(skipped, ", "))
		}
		return registry.Repo{}, errors.New(msg)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		return registry.Repo{}, fmt.Errorf("%w %s: %s", errMultipleRepoMatches, orgRepo, strings.Join(names, ", "))
	}
}
