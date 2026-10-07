package hooks

import (
	"context"
	"fmt"

	"github.com/raphi011/wt/internal/config"
)

// Operation describes one command acting on one worktree, for the hooks that
// run around it.
type Operation struct {
	Hooks     config.HooksConfig
	ConfigDir string // absolute path to ~/.wt/ config directory

	Trigger CommandType
	Action  string // checkout subtype: create, open, pr, manual (for wt hook)

	RepoDir     string // absolute main repo path
	Repo        string // registered repo name
	WorktreeDir string // absolute worktree path, working directory of the hooks
	Branch      string
	PRNumber    *int   // PR/MR number (nil for non-PR checkouts)
	PRRepo      string // forge repo path, e.g. owner/repo

	// BeforeWorkDir overrides the working directory of before-hooks, for
	// commands that create the worktree. Empty = WorktreeDir.
	BeforeWorkDir string
	// AfterWorkDir overrides the working directory of after-hooks, for
	// commands that remove the worktree. Empty = WorktreeDir.
	AfterWorkDir string

	HookNames []string          // --hook flag values
	NoHook    bool              // --no-hook flag
	Env       map[string]string // custom variables from --arg key=value flags
}

// Context returns the placeholder values of the operation for a phase.
func (op Operation) Context(phase PhaseType) Context {
	return Context{
		WorktreeDir: op.WorktreeDir,
		RepoDir:     op.RepoDir,
		Branch:      op.Branch,
		Repo:        op.Repo,
		Trigger:     string(op.Trigger),
		Action:      op.Action,
		Phase:       phase,
		ConfigDir:   op.ConfigDir,
		PRNumber:    op.PRNumber,
		PRRepo:      op.PRRepo,
		Env:         op.Env,
	}
}

// Run runs before-hooks, then fn, then after-hooks.
// If before-hooks fail, fn is not called and the error is returned.
// After-hook failures are logged as warnings.
func (op Operation) Run(ctx context.Context, fn func() error) error {
	// Before hooks (can abort)
	beforeMatches, err := op.selectHooks(PhaseBefore)
	if err != nil {
		return err
	}
	beforeWorkDir := op.WorktreeDir
	if op.BeforeWorkDir != "" {
		beforeWorkDir = op.BeforeWorkDir
	}
	if err := RunBeforeHooks(ctx, beforeMatches, op.Context(PhaseBefore), beforeWorkDir); err != nil {
		return fmt.Errorf("before-hook aborted %s: %w", op.Trigger, err)
	}

	// Core logic
	if err := fn(); err != nil {
		return err
	}

	// After hooks (non-fatal)
	afterMatches, err := op.selectHooks(PhaseAfter)
	if err != nil {
		return err
	}
	afterWorkDir := op.WorktreeDir
	if op.AfterWorkDir != "" {
		afterWorkDir = op.AfterWorkDir
	}
	RunForEach(ctx, afterMatches, op.Context(PhaseAfter), afterWorkDir)

	return nil
}

func (op Operation) selectHooks(phase PhaseType) ([]HookMatch, error) {
	return SelectHooks(op.Hooks, op.HookNames, op.NoHook, HookSelector{Command: op.Trigger, Action: op.Action, Phase: phase})
}
