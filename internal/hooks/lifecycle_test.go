package hooks

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/fs"
)

// lifecycleDirs holds the directories and the log file of one lifecycle test.
type lifecycleDirs struct {
	Repo, Worktree, Log string
}

func newLifecycleDirs(t *testing.T) lifecycleDirs {
	t.Helper()
	tmpDir := fs.ResolvePath(t.TempDir())
	d := lifecycleDirs{
		Repo:     filepath.Join(tmpDir, "repo"),
		Worktree: filepath.Join(tmpDir, "wt"),
		Log:      filepath.Join(tmpDir, "log.txt"),
	}
	for _, dir := range []string{d.Repo, d.Worktree} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	return d
}

// record returns a hook command appending "<word> <working directory>" to the log.
func (d lifecycleDirs) record(word string) string {
	return "echo \"" + word + " $(pwd -P)\" >> '" + d.Log + "'"
}

// recordFn appends "fn" to the log, standing in for the wrapped command.
func (d lifecycleDirs) recordFn(t *testing.T) func() error {
	return func() error {
		f, err := os.OpenFile(d.Log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("open log: %v", err)
		}
		if _, err := f.WriteString("fn\n"); err != nil {
			t.Fatalf("write log: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close log: %v", err)
		}
		return nil
	}
}

func (d lifecycleDirs) lines(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(d.Log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestOperationRun_PhaseOrder(t *testing.T) {
	t.Parallel()

	d := newLifecycleDirs(t)
	var buf bytes.Buffer
	op := Operation{
		Hooks: config.HooksConfig{Hooks: map[string]config.Hook{
			"a-after":  {Command: d.record("after"), On: []string{"checkout"}},
			"z-before": {Command: d.record("before"), On: []string{"before:checkout"}},
			"other":    {Command: d.record("other"), On: []string{"prune"}},
			"manual":   {Command: d.record("manual")},
		}},
		Trigger:     CommandCheckout,
		Action:      ActionCreate,
		WorktreeDir: d.Worktree,
	}

	if err := op.Run(logCtx(&buf), d.recordFn(t)); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	want := []string{"before " + d.Worktree, "fn", "after " + d.Worktree}
	if got := d.lines(t); !slices.Equal(got, want) {
		t.Errorf("log = %q, want %q", got, want)
	}
}

func TestOperationRun_WorkDirPerPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		beforeWorkDir bool
		afterWorkDir  bool
	}{
		{name: "worktree for both phases"},
		{name: "before-hooks outside the worktree", beforeWorkDir: true},
		{name: "after-hooks outside the worktree", afterWorkDir: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := newLifecycleDirs(t)
			var buf bytes.Buffer
			op := Operation{
				Hooks: config.HooksConfig{Hooks: map[string]config.Hook{
					"before": {Command: d.record("before"), On: []string{"before:merge"}},
					"after":  {Command: d.record("after"), On: []string{"merge"}},
				}},
				Trigger:     CommandMerge,
				WorktreeDir: d.Worktree,
			}
			wantBefore, wantAfter := d.Worktree, d.Worktree
			fn := func() error { return nil }
			if tt.beforeWorkDir {
				op.BeforeWorkDir = d.Repo
				wantBefore = d.Repo
			}
			if tt.afterWorkDir {
				op.AfterWorkDir = d.Repo
				wantAfter = d.Repo
				// fn removes the worktree, like `wt pr merge` does after merging.
				fn = func() error { return os.Remove(d.Worktree) }
			}

			if err := op.Run(logCtx(&buf), fn); err != nil {
				t.Fatalf("Run() = %v, want nil", err)
			}

			want := []string{"before " + wantBefore, "after " + wantAfter}
			if got := d.lines(t); !slices.Equal(got, want) {
				t.Errorf("log = %q, want %q", got, want)
			}
		})
	}
}

func TestOperationRun_BeforeHookAborts(t *testing.T) {
	t.Parallel()

	d := newLifecycleDirs(t)
	var buf bytes.Buffer
	op := Operation{
		Hooks: config.HooksConfig{Hooks: map[string]config.Hook{
			"guard": {Command: "sh -c 'exit 3'", On: []string{"before:prune"}},
			"after": {Command: d.record("after"), On: []string{"prune"}},
		}},
		Trigger:     CommandPrune,
		WorktreeDir: d.Worktree,
	}

	err := op.Run(logCtx(&buf), d.recordFn(t))
	if err == nil {
		t.Fatal("Run() = nil, want error")
	}
	if !strings.Contains(err.Error(), "before-hook aborted prune") || !strings.Contains(err.Error(), "exit 3") {
		t.Errorf("error = %q, want before-hook abort with exit code", err.Error())
	}
	if got := d.lines(t); got != nil {
		t.Errorf("log = %q, want nothing to run after a failed before-hook", got)
	}
}

func TestOperationRun_FnErrorSkipsAfterHooks(t *testing.T) {
	t.Parallel()

	d := newLifecycleDirs(t)
	var buf bytes.Buffer
	op := Operation{
		Hooks: config.HooksConfig{Hooks: map[string]config.Hook{
			"before": {Command: d.record("before"), On: []string{"before:checkout"}},
			"after":  {Command: d.record("after"), On: []string{"checkout"}},
		}},
		Trigger:     CommandCheckout,
		WorktreeDir: d.Worktree,
	}
	fnErr := errors.New("fn failed")

	err := op.Run(logCtx(&buf), func() error { return fnErr })
	if !errors.Is(err, fnErr) {
		t.Fatalf("Run() = %v, want %v", err, fnErr)
	}

	want := []string{"before " + d.Worktree}
	if got := d.lines(t); !slices.Equal(got, want) {
		t.Errorf("log = %q, want %q", got, want)
	}
}

func TestOperationRun_AfterHookFailureIsWarning(t *testing.T) {
	t.Parallel()

	d := newLifecycleDirs(t)
	var buf bytes.Buffer
	op := Operation{
		Hooks: config.HooksConfig{Hooks: map[string]config.Hook{
			"a-fail": {Command: "sh -c 'exit 1'", On: []string{"checkout"}},
			"b-next": {Command: d.record("next"), On: []string{"checkout"}},
		}},
		Trigger:     CommandCheckout,
		WorktreeDir: d.Worktree,
		Branch:      "feature",
	}

	if err := op.Run(logCtx(&buf), func() error { return nil }); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	if !strings.Contains(buf.String(), "Warning: hook \"a-fail\" failed for feature") {
		t.Errorf("output = %q, want warning for the failed hook", buf.String())
	}
	want := []string{"next " + d.Worktree}
	if got := d.lines(t); !slices.Equal(got, want) {
		t.Errorf("log = %q, want %q", got, want)
	}
}

func TestOperationRun_ExplicitHooks(t *testing.T) {
	t.Parallel()

	d := newLifecycleDirs(t)
	var buf bytes.Buffer
	op := Operation{
		Hooks: config.HooksConfig{Hooks: map[string]config.Hook{
			"before": {Command: d.record("before"), On: []string{"before:checkout"}},
			"after":  {Command: d.record("after"), On: []string{"checkout"}},
			"named":  {Command: d.record("named")},
		}},
		Trigger:     CommandCheckout,
		WorktreeDir: d.Worktree,
		HookNames:   []string{"named"},
	}

	if err := op.Run(logCtx(&buf), d.recordFn(t)); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	// Named hooks replace the hooks matched by "on" and run once, after fn.
	want := []string{"fn", "named " + d.Worktree}
	if got := d.lines(t); !slices.Equal(got, want) {
		t.Errorf("log = %q, want %q", got, want)
	}
}

func TestOperationRun_NoHook(t *testing.T) {
	t.Parallel()

	d := newLifecycleDirs(t)
	var buf bytes.Buffer
	op := Operation{
		Hooks: config.HooksConfig{Hooks: map[string]config.Hook{
			"before": {Command: d.record("before"), On: []string{"before:checkout"}},
			"after":  {Command: d.record("after"), On: []string{"checkout"}},
		}},
		Trigger:     CommandCheckout,
		WorktreeDir: d.Worktree,
		NoHook:      true,
	}

	if err := op.Run(logCtx(&buf), d.recordFn(t)); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	want := []string{"fn"}
	if got := d.lines(t); !slices.Equal(got, want) {
		t.Errorf("log = %q, want %q", got, want)
	}
}

func TestOperationRun_Placeholders(t *testing.T) {
	t.Parallel()

	d := newLifecycleDirs(t)
	var buf bytes.Buffer
	values := "{phase} {trigger} {action} {repo} {branch} {pr-number} {pr-repo} {ticket} {repo-dir} {worktree-dir} {config-dir}"
	op := Operation{
		Hooks: config.HooksConfig{Hooks: map[string]config.Hook{
			"before": {Command: "echo '" + values + "' >> '" + d.Log + "'", On: []string{"before:checkout:pr"}},
			"after":  {Command: "echo '" + values + "' >> '" + d.Log + "'", On: []string{"checkout:pr"}},
		}},
		ConfigDir:   "/cfg",
		Trigger:     CommandCheckout,
		Action:      ActionPR,
		RepoDir:     d.Repo,
		Repo:        "myrepo",
		WorktreeDir: d.Worktree,
		Branch:      "feature",
		PRNumber:    new(42),
		PRRepo:      "owner/myrepo",
		Env:         map[string]string{"ticket": "T-1"},
	}

	if err := op.Run(logCtx(&buf), func() error { return nil }); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	rest := " checkout pr myrepo feature 42 owner/myrepo T-1 " + d.Repo + " " + d.Worktree + " /cfg"
	want := []string{"before" + rest, "after" + rest}
	if got := d.lines(t); !slices.Equal(got, want) {
		t.Errorf("log = %q, want %q", got, want)
	}
}

func TestOperationContext(t *testing.T) {
	t.Parallel()

	env := map[string]string{"key": "value"}
	op := Operation{
		ConfigDir:   "/cfg",
		Trigger:     CommandRun,
		Action:      ActionManual,
		RepoDir:     "/repo",
		Repo:        "myrepo",
		WorktreeDir: "/wt",
		Branch:      "feature",
		Env:         env,
	}

	got := op.Context(PhaseAfter)

	if got.Phase != PhaseAfter || got.Trigger != "run" || got.Action != ActionManual {
		t.Errorf("phase/trigger/action = %q/%q/%q, want after/run/manual", got.Phase, got.Trigger, got.Action)
	}
	if got.RepoDir != "/repo" || got.Repo != "myrepo" || got.WorktreeDir != "/wt" || got.Branch != "feature" {
		t.Errorf("repo/worktree = %q/%q/%q/%q, want /repo/myrepo//wt/feature", got.RepoDir, got.Repo, got.WorktreeDir, got.Branch)
	}
	if got.ConfigDir != "/cfg" || got.Env["key"] != "value" || got.PRNumber != nil || got.DryRun {
		t.Errorf("Context() = %+v, want config dir, env, no PR, no dry-run", got)
	}
}
