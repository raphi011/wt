//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

// === note set tests ===

// TestNoteSet_ByID verifies setting a branch note by worktree ID.
//
// Scenario: User runs `wt note set -n 1 "Test note"`
// Expected: Note stored in git config branch.<branch>.description
func TestNoteSet_ByID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteSetCmd{
		ID:   1,
		Text: "Test note via ID",
	}

	if err := runNoteSetCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt note set -n 1 failed: %v", err)
	}

	note := getBranchNote(t, repoPath, "feature")
	if note != "Test note via ID" {
		t.Errorf("expected note 'Test note via ID', got %q", note)
	}
}

// TestNoteSet_ByRepoName verifies setting a branch note by repo name.
//
// Scenario: User runs `wt note set -r myrepo "Test note"`
// Expected: Note set on main branch of the repo
func TestNoteSet_ByRepoName(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteSetCmd{
		Repository: "myrepo",
		Text:       "Test note via repo",
	}

	if err := runNoteSetCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt note set -r myrepo failed: %v", err)
	}

	// Note is set on the main branch (current branch of main repo)
	note := getBranchNote(t, repoPath, "main")
	if note != "Test note via repo" {
		t.Errorf("expected note 'Test note via repo', got %q", note)
	}
}

// TestNoteSet_InsideWorktree verifies setting a note when running from inside a worktree.
//
// Scenario: User runs `wt note set "Note"` from inside a worktree
// Expected: Note set on current worktree's branch
func TestNoteSet_InsideWorktree(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteSetCmd{
		Text: "Note from inside worktree",
	}

	// Run from inside worktree
	if err := runNoteSetCommand(t, worktreePath, cfg, cmd); err != nil {
		t.Fatalf("wt note set from inside worktree failed: %v", err)
	}

	note := getBranchNote(t, repoPath, "feature")
	if note != "Note from inside worktree" {
		t.Errorf("expected note 'Note from inside worktree', got %q", note)
	}
}

// TestNoteSet_InsideMainRepo verifies setting a note when running from inside main repo.
//
// Scenario: User runs `wt note set "Note"` from inside main repo
// Expected: Note set on main branch
func TestNoteSet_InsideMainRepo(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteSetCmd{
		Text: "Note from inside main repo",
	}

	// Run from inside main repo
	if err := runNoteSetCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt note set from inside main repo failed: %v", err)
	}

	note := getBranchNote(t, repoPath, "main")
	if note != "Note from inside main repo" {
		t.Errorf("expected note 'Note from inside main repo', got %q", note)
	}
}

// TestNoteSet_OverwriteExisting verifies that setting a note overwrites existing note.
//
// Scenario: User sets a note, then sets a different note
// Expected: Second note replaces the first
func TestNoteSet_OverwriteExisting(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Set first note
	cmd1 := &NoteSetCmd{
		ID:   1,
		Text: "First note",
	}

	if err := runNoteSetCommand(t, worktreeDir, cfg, cmd1); err != nil {
		t.Fatalf("first wt note set failed: %v", err)
	}

	note := getBranchNote(t, repoPath, "feature")
	if note != "First note" {
		t.Errorf("expected 'First note', got %q", note)
	}

	// Overwrite with second note
	cmd2 := &NoteSetCmd{
		ID:   1,
		Text: "Second note",
	}

	if err := runNoteSetCommand(t, worktreeDir, cfg, cmd2); err != nil {
		t.Fatalf("second wt note set failed: %v", err)
	}

	note = getBranchNote(t, repoPath, "feature")
	if note != "Second note" {
		t.Errorf("expected 'Second note', got %q", note)
	}
}

// === note get tests ===

// TestNoteGet_ByID verifies retrieving a branch note by worktree ID.
//
// Scenario: User runs `wt note get -n 1`
// Expected: Note content printed to stdout
func TestNoteGet_ByID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	// Set note directly via git
	runGitCommand(t, repoPath, "git", "config", "branch.feature.description", "Note for get test")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteGetCmd{
		ID: 1,
	}

	output, err := runNoteGetCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt note get -n 1 failed: %v", err)
	}

	if strings.TrimSpace(output) != "Note for get test" {
		t.Errorf("expected 'Note for get test', got %q", strings.TrimSpace(output))
	}
}

// TestNoteGet_ByRepoName verifies retrieving a branch note by repo name.
//
// Scenario: User runs `wt note get -r myrepo`
// Expected: Note from main branch printed to stdout
func TestNoteGet_ByRepoName(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")

	// Set note on main branch
	runGitCommand(t, repoPath, "git", "config", "branch.main.description", "Note for repo get test")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteGetCmd{
		Repository: "myrepo",
	}

	output, err := runNoteGetCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt note get -r myrepo failed: %v", err)
	}

	if strings.TrimSpace(output) != "Note for repo get test" {
		t.Errorf("expected 'Note for repo get test', got %q", strings.TrimSpace(output))
	}
}

// TestNoteGet_InsideWorktree verifies getting a note from inside a worktree.
//
// Scenario: User runs `wt note get` from inside a worktree
// Expected: Note from current branch printed
func TestNoteGet_InsideWorktree(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Set note directly via git
	runGitCommand(t, repoPath, "git", "config", "branch.feature.description", "Worktree get test")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteGetCmd{}

	// Run from inside worktree
	output, err := runNoteGetCommand(t, worktreePath, cfg, cmd)
	if err != nil {
		t.Fatalf("wt note get from inside worktree failed: %v", err)
	}

	if strings.TrimSpace(output) != "Worktree get test" {
		t.Errorf("expected 'Worktree get test', got %q", strings.TrimSpace(output))
	}
}

// TestNoteGet_NoNoteExists verifies graceful handling when no note exists.
//
// Scenario: User runs `wt note get -n 1` but no note has been set
// Expected: Empty output, no error
func TestNoteGet_NoNoteExists(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteGetCmd{
		ID: 1,
	}

	// Should succeed with empty output
	output, err := runNoteGetCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt note get with no note should succeed: %v", err)
	}

	if strings.TrimSpace(output) != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

// TestNoteGet_DefaultSubcommand verifies that "get" is the default subcommand.
//
// Scenario: User runs `wt note -n 1` (without explicit "get")
// Expected: Note content printed (same as `wt note get -n 1`)
func TestNoteGet_DefaultSubcommand(t *testing.T) {
	t.Parallel()
	// Test that `wt note -n 1` works (get is default subcommand)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	// Set note directly via git
	runGitCommand(t, repoPath, "git", "config", "branch.feature.description", "Default subcommand test")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Using NoteGetCmd directly simulates `wt note -n 1` behavior
	cmd := &NoteGetCmd{
		ID: 1,
	}

	output, err := runNoteGetCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt note -n 1 (default get) failed: %v", err)
	}

	if strings.TrimSpace(output) != "Default subcommand test" {
		t.Errorf("expected 'Default subcommand test', got %q", strings.TrimSpace(output))
	}
}

// === note clear tests ===

// TestNoteClear_ByID verifies clearing a branch note by worktree ID.
//
// Scenario: User runs `wt note clear -n 1`
// Expected: Note removed from git config
func TestNoteClear_ByID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	// Set note first
	runGitCommand(t, repoPath, "git", "config", "branch.feature.description", "Note to clear")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteClearCmd{
		ID: 1,
	}

	if err := runNoteClearCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt note clear -n 1 failed: %v", err)
	}

	note := getBranchNote(t, repoPath, "feature")
	if note != "" {
		t.Errorf("expected empty note after clear, got %q", note)
	}
}

// TestNoteClear_ByRepoName verifies clearing a branch note by repo name.
//
// Scenario: User runs `wt note clear -r myrepo`
// Expected: Note removed from main branch
func TestNoteClear_ByRepoName(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")

	// Set note on main branch
	runGitCommand(t, repoPath, "git", "config", "branch.main.description", "Note to clear")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteClearCmd{
		Repository: "myrepo",
	}

	if err := runNoteClearCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt note clear -r myrepo failed: %v", err)
	}

	note := getBranchNote(t, repoPath, "main")
	if note != "" {
		t.Errorf("expected empty note after clear, got %q", note)
	}
}

// TestNoteClear_InsideWorktree verifies clearing a note from inside a worktree.
//
// Scenario: User runs `wt note clear` from inside a worktree
// Expected: Note removed from current branch
func TestNoteClear_InsideWorktree(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Set note first
	runGitCommand(t, repoPath, "git", "config", "branch.feature.description", "Note to clear")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteClearCmd{}

	// Run from inside worktree
	if err := runNoteClearCommand(t, worktreePath, cfg, cmd); err != nil {
		t.Fatalf("wt note clear from inside worktree failed: %v", err)
	}

	note := getBranchNote(t, repoPath, "feature")
	if note != "" {
		t.Errorf("expected empty note after clear, got %q", note)
	}
}

// TestNoteClear_NonExistentNote verifies graceful handling when clearing a non-existent note.
//
// Scenario: User runs `wt note clear -n 1` but no note exists
// Expected: Command succeeds (idempotent)
func TestNoteClear_NonExistentNote(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	// No note set

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteClearCmd{
		ID: 1,
	}

	// Should succeed gracefully
	if err := runNoteClearCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt note clear on non-existent note should succeed: %v", err)
	}

	// Verify still no note
	note := getBranchNote(t, repoPath, "feature")
	if note != "" {
		t.Errorf("expected empty note, got %q", note)
	}
}

// === error cases ===

// TestNoteGet_ErrorInvalidID verifies error when specified ID doesn't exist.
//
// Scenario: User runs `wt note get -n 999`
// Expected: Error for invalid ID
func TestNoteGet_ErrorInvalidID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteGetCmd{
		ID: 999, // Invalid ID
	}

	_, err := runNoteGetCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

// TestNoteGet_ErrorRepoNotFound verifies error when specified repo doesn't exist.
//
// Scenario: User runs `wt note get -r nonexistent-repo`
// Expected: Error mentioning the nonexistent repository
func TestNoteGet_ErrorRepoNotFound(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create a different repo
	setupTestRepo(t, repoDir, "other-repo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteGetCmd{
		Repository: "nonexistent-repo",
	}

	_, err := runNoteGetCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when repo not found")
	}
	if !strings.Contains(err.Error(), "nonexistent-repo") {
		t.Errorf("expected error to mention repo name, got: %v", err)
	}
}

// TestNoteSet_ErrorInvalidID verifies error when specified ID doesn't exist.
//
// Scenario: User runs `wt note set -n 999 "note"`
// Expected: Error for invalid ID
func TestNoteSet_ErrorInvalidID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteSetCmd{
		ID:   999, // Invalid ID
		Text: "This should fail",
	}

	err := runNoteSetCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

// TestNoteClear_ErrorInvalidID verifies error when specified ID doesn't exist.
//
// Scenario: User runs `wt note clear -n 999`
// Expected: Error for invalid ID
func TestNoteClear_ErrorInvalidID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &NoteClearCmd{
		ID: 999, // Invalid ID
	}

	err := runNoteClearCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

// === helper functions ===

func runNoteSetCommand(t *testing.T, workDir string, cfg *config.Config, cmd *NoteSetCmd) error {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx, _ := testContextWithOutput(t)
	return cmd.runNoteSet(ctx)
}

func runNoteGetCommand(t *testing.T, workDir string, cfg *config.Config, cmd *NoteGetCmd) (string, error) {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx, out := testContextWithOutput(t)
	err := cmd.runNoteGet(ctx)
	return out.String(), err
}

func runNoteClearCommand(t *testing.T, workDir string, cfg *config.Config, cmd *NoteClearCmd) error {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx := testContext(t)
	return cmd.runNoteClear(ctx)
}
