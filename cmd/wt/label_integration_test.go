//go:build integration

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
)

// --- Label Add Tests ---

// TestLabelAdd_CurrentRepo verifies adding a label to the current repo
// when running from inside it.
//
// Scenario: User runs `wt label add backend` from inside a repo
// Expected: Label "backend" added to the repo's git config
func TestLabelAdd_CurrentRepo(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{}
	cmd := &LabelAddCmd{Label: "backend"}

	if err := runLabelAddCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label add failed: %v", err)
	}

	labels := getRepoLabels(t, repoPath)
	if !slices.Contains(labels, "backend") {
		t.Errorf("expected label 'backend', got %v", labels)
	}
}

// TestLabelAdd_ByRepoName verifies adding a label using -r flag
// to specify the target repo by name.
//
// Scenario: User runs `wt label add frontend -r myrepo` from outside repo
// Expected: Label "frontend" added to myrepo's git config
func TestLabelAdd_ByRepoName(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelAddCmd{
		Label:      "frontend",
		Repository: []string{"myrepo"},
	}

	// Run from outside any repo
	if err := runLabelAddCommand(t, repoDir, cfg, cmd); err != nil {
		t.Fatalf("wt label add -r myrepo failed: %v", err)
	}

	labels := getRepoLabels(t, repoPath)
	if !slices.Contains(labels, "frontend") {
		t.Errorf("expected label 'frontend', got %v", labels)
	}
}

// TestLabelAdd_MultipleRepos verifies adding the same label to multiple repos
// using repeated -r flags.
//
// Scenario: User runs `wt label add shared -r repo-a -r repo-b`
// Expected: Label "shared" added to both repo-a and repo-b
func TestLabelAdd_MultipleRepos(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelAddCmd{
		Label:      "shared",
		Repository: []string{"repo-a", "repo-b"},
	}

	if err := runLabelAddCommand(t, repoDir, cfg, cmd); err != nil {
		t.Fatalf("wt label add -r repo-a -r repo-b failed: %v", err)
	}

	labelsA := getRepoLabels(t, repoA)
	if !slices.Contains(labelsA, "shared") {
		t.Errorf("repo-a expected label 'shared', got %v", labelsA)
	}

	labelsB := getRepoLabels(t, repoB)
	if !slices.Contains(labelsB, "shared") {
		t.Errorf("repo-b expected label 'shared', got %v", labelsB)
	}
}

// TestLabelAdd_DuplicateIsIdempotent verifies that adding an existing label
// is a no-op (idempotent) - no error and no duplicate created.
//
// Scenario: User runs `wt label add existing` when "existing" label already set
// Expected: Command succeeds, label appears exactly once
func TestLabelAdd_DuplicateIsIdempotent(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	// Set label directly
	setRepoLabel(t, repoPath, "existing")

	cfg := &config.Config{}
	cmd := &LabelAddCmd{Label: "existing"}

	// Adding again should succeed
	if err := runLabelAddCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label add (duplicate) failed: %v", err)
	}

	labels := getRepoLabels(t, repoPath)
	// Should still have only one "existing"
	count := 0
	for _, l := range labels {
		if l == "existing" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'existing' label, got %d in %v", count, labels)
	}
}

// TestLabelAdd_AddsToExistingLabels verifies that adding a label preserves
// existing labels (append, not replace).
//
// Scenario: Repo has "first" label, user runs `wt label add second`
// Expected: Both "first" and "second" labels present
func TestLabelAdd_AddsToExistingLabels(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	// Set initial label
	setRepoLabel(t, repoPath, "first")

	cfg := &config.Config{}
	cmd := &LabelAddCmd{Label: "second"}

	if err := runLabelAddCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label add (second) failed: %v", err)
	}

	labels := getRepoLabels(t, repoPath)
	if !slices.Contains(labels, "first") {
		t.Errorf("expected label 'first' to be preserved, got %v", labels)
	}
	if !slices.Contains(labels, "second") {
		t.Errorf("expected label 'second' to be added, got %v", labels)
	}
}

// TestLabelAdd_ErrorOutsideRepoWithoutFlag verifies error when running
// outside a repo without specifying -r flag.
//
// Scenario: User runs `wt label add backend` from non-repo directory
// Expected: Error about requiring -r/--repository flag
func TestLabelAdd_ErrorOutsideRepoWithoutFlag(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	cfg := &config.Config{}
	cmd := &LabelAddCmd{Label: "backend"}

	err := runLabelAddCommand(t, tempDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when outside repo without -r flag")
	}
	if !strings.Contains(err.Error(), "-r/--repository") {
		t.Errorf("expected error about -r flag, got: %v", err)
	}
}

// TestLabelAdd_ErrorRepoNotFound verifies error when specified repo
// doesn't exist in repo_dir.
//
// Scenario: User runs `wt label add backend -r nonexistent`
// Expected: Error about no valid repositories found
func TestLabelAdd_ErrorRepoNotFound(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelAddCmd{
		Label:      "backend",
		Repository: []string{"nonexistent"},
	}

	err := runLabelAddCommand(t, repoDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when repo not found")
	}
	if !strings.Contains(err.Error(), "no valid repositories") {
		t.Errorf("expected 'no valid repositories' error, got: %v", err)
	}
}

// --- Label Remove Tests ---

// TestLabelRemove_CurrentRepo verifies removing a label from the current repo
// when running from inside it.
//
// Scenario: User runs `wt label remove toremove` from inside repo with that label
// Expected: Label "toremove" removed from repo's git config
func TestLabelRemove_CurrentRepo(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	// Set label first
	setRepoLabel(t, repoPath, "toremove")

	cfg := &config.Config{}
	cmd := &LabelRemoveCmd{Label: "toremove"}

	if err := runLabelRemoveCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label remove failed: %v", err)
	}

	labels := getRepoLabels(t, repoPath)
	if slices.Contains(labels, "toremove") {
		t.Errorf("label 'toremove' should be removed, got %v", labels)
	}
}

// TestLabelRemove_ByRepoName verifies removing a label using -r flag
// to specify the target repo by name.
//
// Scenario: User runs `wt label remove toremove -r myrepo`
// Expected: Label "toremove" removed from myrepo's git config
func TestLabelRemove_ByRepoName(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	setRepoLabel(t, repoPath, "toremove")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelRemoveCmd{
		Label:      "toremove",
		Repository: []string{"myrepo"},
	}

	if err := runLabelRemoveCommand(t, repoDir, cfg, cmd); err != nil {
		t.Fatalf("wt label remove -r myrepo failed: %v", err)
	}

	labels := getRepoLabels(t, repoPath)
	if slices.Contains(labels, "toremove") {
		t.Errorf("label 'toremove' should be removed, got %v", labels)
	}
}

// TestLabelRemove_MultipleRepos verifies removing a label from multiple repos
// using repeated -r flags.
//
// Scenario: User runs `wt label remove shared -r repo-a -r repo-b`
// Expected: Label "shared" removed from both repo-a and repo-b
func TestLabelRemove_MultipleRepos(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	setRepoLabel(t, repoA, "shared")
	setRepoLabel(t, repoB, "shared")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelRemoveCmd{
		Label:      "shared",
		Repository: []string{"repo-a", "repo-b"},
	}

	if err := runLabelRemoveCommand(t, repoDir, cfg, cmd); err != nil {
		t.Fatalf("wt label remove -r repo-a -r repo-b failed: %v", err)
	}

	labelsA := getRepoLabels(t, repoA)
	if slices.Contains(labelsA, "shared") {
		t.Errorf("repo-a: label 'shared' should be removed, got %v", labelsA)
	}

	labelsB := getRepoLabels(t, repoB)
	if slices.Contains(labelsB, "shared") {
		t.Errorf("repo-b: label 'shared' should be removed, got %v", labelsB)
	}
}

// TestLabelRemove_NonexistentLabel verifies that removing a label that
// doesn't exist is a no-op (succeeds without error).
//
// Scenario: User runs `wt label remove nonexistent` on repo without that label
// Expected: Command succeeds with no-op
func TestLabelRemove_NonexistentLabel(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{}
	cmd := &LabelRemoveCmd{Label: "nonexistent"}

	// Should succeed even if label doesn't exist
	if err := runLabelRemoveCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label remove (nonexistent) should succeed: %v", err)
	}
}

// TestLabelRemove_PreservesOtherLabels verifies that removing a label
// preserves other labels on the repo.
//
// Scenario: Repo has "keep", "remove", "also-keep", user removes "remove"
// Expected: "keep" and "also-keep" preserved, "remove" gone
func TestLabelRemove_PreservesOtherLabels(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	// Set multiple labels
	setRepoLabels(t, repoPath, "keep", "remove", "also-keep")

	cfg := &config.Config{}
	cmd := &LabelRemoveCmd{Label: "remove"}

	if err := runLabelRemoveCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label remove failed: %v", err)
	}

	labels := getRepoLabels(t, repoPath)
	if !slices.Contains(labels, "keep") {
		t.Errorf("label 'keep' should be preserved, got %v", labels)
	}
	if !slices.Contains(labels, "also-keep") {
		t.Errorf("label 'also-keep' should be preserved, got %v", labels)
	}
	if slices.Contains(labels, "remove") {
		t.Errorf("label 'remove' should be removed, got %v", labels)
	}
}

// --- Label List Tests ---

// TestLabelList_CurrentRepo verifies listing labels from the current repo
// when running from inside it.
//
// Scenario: User runs `wt label list` from inside repo with labels
// Expected: Command succeeds, outputs repo labels
func TestLabelList_CurrentRepo(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	setRepoLabels(t, repoPath, "backend", "api")

	cfg := &config.Config{}
	cmd := &LabelListCmd{}

	// Capture output - just verify no error
	if err := runLabelListCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label list failed: %v", err)
	}
}

// TestLabelList_ByRepoName verifies listing labels using -r flag
// to specify the target repo by name.
//
// Scenario: User runs `wt label list -r myrepo`
// Expected: Command succeeds, outputs myrepo's labels
func TestLabelList_ByRepoName(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelListCmd{Repository: []string{"myrepo"}}

	if err := runLabelListCommand(t, repoDir, cfg, cmd); err != nil {
		t.Fatalf("wt label list -r myrepo failed: %v", err)
	}
}

// TestLabelList_MultipleRepos verifies listing labels from multiple repos
// using repeated -r flags.
//
// Scenario: User runs `wt label list -r repo-a -r repo-b`
// Expected: Command succeeds, outputs labels for both repos
func TestLabelList_MultipleRepos(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	setRepoLabel(t, repoA, "frontend")
	setRepoLabel(t, repoB, "backend")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelListCmd{Repository: []string{"repo-a", "repo-b"}}

	if err := runLabelListCommand(t, repoDir, cfg, cmd); err != nil {
		t.Fatalf("wt label list -r repo-a -r repo-b failed: %v", err)
	}
}

// TestLabelList_Global verifies listing labels from all repos using -g flag.
//
// Scenario: User runs `wt label list -g` with 3 repos (2 with labels)
// Expected: Command succeeds, outputs labels from all repos
func TestLabelList_Global(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")
	setupTestRepo(t, repoDir, "repo-c") // No labels

	setRepoLabel(t, repoA, "frontend")
	setRepoLabel(t, repoB, "backend")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelListCmd{Global: true}

	if err := runLabelListCommand(t, repoDir, cfg, cmd); err != nil {
		t.Fatalf("wt label list -g failed: %v", err)
	}
}

// TestLabelList_NoLabels verifies that listing labels on repo with no labels
// succeeds with empty output.
//
// Scenario: User runs `wt label list` on repo without any labels
// Expected: Command succeeds, no output
func TestLabelList_NoLabels(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{}
	cmd := &LabelListCmd{}

	// Should succeed with no output
	if err := runLabelListCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label list (no labels) failed: %v", err)
	}
}

// TestLabelList_ErrorOutsideRepoWithoutFlag verifies error when running
// outside a repo without specifying -r or -g flag.
//
// Scenario: User runs `wt label list` from non-repo directory
// Expected: Error about requiring -r or -g flag
func TestLabelList_ErrorOutsideRepoWithoutFlag(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	cfg := &config.Config{}
	cmd := &LabelListCmd{}

	err := runLabelListCommand(t, tempDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when outside repo without -r or -g flag")
	}
}

// --- Label Clear Tests ---

// TestLabelClear_CurrentRepo verifies clearing all labels from the current repo
// when running from inside it.
//
// Scenario: User runs `wt label clear` from inside repo with 3 labels
// Expected: All labels removed from repo's git config
func TestLabelClear_CurrentRepo(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	setRepoLabels(t, repoPath, "label1", "label2", "label3")

	cfg := &config.Config{}
	cmd := &LabelClearCmd{}

	if err := runLabelClearCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label clear failed: %v", err)
	}

	labels := getRepoLabels(t, repoPath)
	if len(labels) != 0 {
		t.Errorf("expected no labels after clear, got %v", labels)
	}
}

// TestLabelClear_ByRepoName verifies clearing labels using -r flag
// to specify the target repo by name.
//
// Scenario: User runs `wt label clear -r myrepo`
// Expected: All labels removed from myrepo's git config
func TestLabelClear_ByRepoName(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	setRepoLabels(t, repoPath, "label1", "label2")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelClearCmd{Repository: []string{"myrepo"}}

	if err := runLabelClearCommand(t, repoDir, cfg, cmd); err != nil {
		t.Fatalf("wt label clear -r myrepo failed: %v", err)
	}

	labels := getRepoLabels(t, repoPath)
	if len(labels) != 0 {
		t.Errorf("expected no labels after clear, got %v", labels)
	}
}

// TestLabelClear_MultipleRepos verifies clearing labels from multiple repos
// using repeated -r flags.
//
// Scenario: User runs `wt label clear -r repo-a -r repo-b`
// Expected: All labels removed from both repos
func TestLabelClear_MultipleRepos(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	setRepoLabel(t, repoA, "frontend")
	setRepoLabel(t, repoB, "backend")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &LabelClearCmd{Repository: []string{"repo-a", "repo-b"}}

	if err := runLabelClearCommand(t, repoDir, cfg, cmd); err != nil {
		t.Fatalf("wt label clear -r repo-a -r repo-b failed: %v", err)
	}

	labelsA := getRepoLabels(t, repoA)
	if len(labelsA) != 0 {
		t.Errorf("repo-a: expected no labels, got %v", labelsA)
	}

	labelsB := getRepoLabels(t, repoB)
	if len(labelsB) != 0 {
		t.Errorf("repo-b: expected no labels, got %v", labelsB)
	}
}

// TestLabelClear_NoLabels verifies that clearing labels on repo with no labels
// succeeds as a no-op.
//
// Scenario: User runs `wt label clear` on repo without any labels
// Expected: Command succeeds with no-op
func TestLabelClear_NoLabels(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{}
	cmd := &LabelClearCmd{}

	// Should succeed even if no labels exist
	if err := runLabelClearCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt label clear (no labels) should succeed: %v", err)
	}
}

// TestLabelClear_ErrorOutsideRepoWithoutFlag verifies error when running
// outside a repo without specifying -r flag.
//
// Scenario: User runs `wt label clear` from non-repo directory
// Expected: Error about requiring -r flag
func TestLabelClear_ErrorOutsideRepoWithoutFlag(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	cfg := &config.Config{}
	cmd := &LabelClearCmd{}

	err := runLabelClearCommand(t, tempDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when outside repo without -r flag")
	}
}

// --- Inside Worktree Tests ---

// TestLabelAdd_InsideWorktree verifies that running label add from inside
// a worktree adds the label to the main repo (not the worktree).
//
// Scenario: User runs `wt label add backend` from inside a worktree
// Expected: Label "backend" added to main repo's git config
func TestLabelAdd_InsideWorktree(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	cfg := &config.Config{}
	cmd := &LabelAddCmd{Label: "backend"}

	// Run from inside worktree - should add label to main repo
	if err := runLabelAddCommand(t, worktreePath, cfg, cmd); err != nil {
		t.Fatalf("wt label add (from worktree) failed: %v", err)
	}

	// Verify label was added to main repo
	labels := getRepoLabels(t, repoPath)
	if !slices.Contains(labels, "backend") {
		t.Errorf("expected label 'backend' on main repo, got %v", labels)
	}
}

// TestLabelList_InsideWorktree verifies that running label list from inside
// a worktree shows labels from the main repo.
//
// Scenario: User runs `wt label list` from inside a worktree
// Expected: Labels from main repo are listed
func TestLabelList_InsideWorktree(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	setRepoLabel(t, repoPath, "backend")

	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	cfg := &config.Config{}
	cmd := &LabelListCmd{}

	// Run from inside worktree - should list labels from main repo
	if err := runLabelListCommand(t, worktreePath, cfg, cmd); err != nil {
		t.Fatalf("wt label list (from worktree) failed: %v", err)
	}
}

// --- Helper Functions ---

func runLabelAddCommand(t *testing.T, workDir string, cfg *config.Config, cmd *LabelAddCmd) error {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx := testContext(t)
	return cmd.runLabelAdd(ctx)
}

func runLabelRemoveCommand(t *testing.T, workDir string, cfg *config.Config, cmd *LabelRemoveCmd) error {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx := testContext(t)
	return cmd.runLabelRemove(ctx)
}

func runLabelListCommand(t *testing.T, workDir string, cfg *config.Config, cmd *LabelListCmd) error {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx := testContext(t)
	return cmd.runLabelList(ctx)
}

func runLabelClearCommand(t *testing.T, workDir string, cfg *config.Config, cmd *LabelClearCmd) error {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx := testContext(t)
	return cmd.runLabelClear(ctx)
}

// getRepoLabels gets all labels from a repo using git config
func getRepoLabels(t *testing.T, repoPath string) []string {
	t.Helper()

	labels, err := git.GetLabels(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("failed to get labels: %v", err)
	}
	return labels
}

// setRepoLabels sets multiple labels on a repo
func setRepoLabels(t *testing.T, repoPath string, labels ...string) {
	t.Helper()

	value := strings.Join(labels, ",")
	cmd := exec.Command("git", "-C", repoPath, "config", "wt.labels", value)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to set labels: %v\n%s", err, out)
	}
}
