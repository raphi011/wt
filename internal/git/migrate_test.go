package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetWorktreeMetadataName(t *testing.T) {
	t.Parallel()

	t.Run("valid worktree", func(t *testing.T) {
		t.Parallel()
		repoPath := setupTestRepo(t)
		tmpDir := filepath.Dir(repoPath)
		ctx := context.Background()

		wtPath := filepath.Join(tmpDir, "wt-meta-test")
		if err := runGit(ctx, repoPath, "worktree", "add", "-b", "meta-branch", wtPath); err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		name, err := getWorktreeMetadataName(ctx, wtPath)
		if err != nil {
			t.Fatalf("getWorktreeMetadataName failed: %v", err)
		}
		if name == "" {
			t.Fatal("expected non-empty metadata name")
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()
		_, err := getWorktreeMetadataName(context.Background(), "/nonexistent/path")
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}

func TestValidateMigration_HappyPath(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test-repo",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	if plan.RepoPath != repoPath {
		t.Errorf("RepoPath = %q, want %q", plan.RepoPath, repoPath)
	}
	if plan.GitDir != filepath.Join(repoPath, ".git") {
		t.Errorf("GitDir = %q, want %q", plan.GitDir, filepath.Join(repoPath, ".git"))
	}
	if plan.CurrentBranch != "main" {
		t.Errorf("CurrentBranch = %q, want main", plan.CurrentBranch)
	}
	if plan.HasSubmodules {
		t.Error("HasSubmodules should be false")
	}
	if len(plan.WorktreesToFix) != 0 {
		t.Errorf("WorktreesToFix should be empty, got %d", len(plan.WorktreesToFix))
	}

	// MainWorktreePath should be nested since format is "{branch}"
	wantMainWT := filepath.Join(repoPath, "main")
	if plan.MainWorktreePath != wantMainWT {
		t.Errorf("MainWorktreePath = %q, want %q", plan.MainWorktreePath, wantMainWT)
	}
}

func TestValidateMigration_WithWorktrees(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	// Create worktrees outside the repo
	wt1 := filepath.Join(tmpDir, "wt-feat-1")
	wt2 := filepath.Join(tmpDir, "wt-feat-2")
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "feature-1", wt1); err != nil {
		t.Fatalf("failed to create worktree 1: %v", err)
	}
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "feature-2", wt2); err != nil {
		t.Fatalf("failed to create worktree 2: %v", err)
	}

	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test-repo",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	if len(plan.WorktreesToFix) != 2 {
		t.Fatalf("WorktreesToFix = %d, want 2", len(plan.WorktreesToFix))
	}

	byBranch := map[string]WorktreeMigration{}
	for _, wt := range plan.WorktreesToFix {
		byBranch[wt.Branch] = wt
	}

	for _, branch := range []string{"feature-1", "feature-2"} {
		wt, ok := byBranch[branch]
		if !ok {
			t.Errorf("expected %s in WorktreesToFix", branch)
			continue
		}
		if !wt.NeedsMove {
			t.Errorf("WorktreesToFix[%s].NeedsMove = false, want true", branch)
		}
		wantNewPath := filepath.Join(repoPath, branch)
		if wt.NewPath != wantNewPath {
			t.Errorf("WorktreesToFix[%s].NewPath = %q, want %q", branch, wt.NewPath, wantNewPath)
		}
		wantOldPath := filepath.Join(tmpDir, "wt-feat-"+branch[len("feature-"):])
		if wt.OldPath != wantOldPath {
			t.Errorf("WorktreesToFix[%s].OldPath = %q, want %q", branch, wt.OldPath, wantOldPath)
		}
	}
}

func TestValidateMigration_AlreadyBare(t *testing.T) {
	t.Parallel()

	tmpDir := resolveTempDir(t)
	ctx := context.Background()

	// Create a directory with .git/ initialized as bare (simulates bare-in-.git)
	wrapperPath := filepath.Join(tmpDir, "wrapper-repo")
	gitDir := filepath.Join(wrapperPath, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	if err := runGit(ctx, gitDir, "init", "--bare"); err != nil {
		t.Fatalf("failed to init bare in .git: %v", err)
	}

	_, err := ValidateMigration(ctx, wrapperPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test",
	})
	if err == nil {
		t.Fatal("expected error for already bare repo")
	}
	if !strings.Contains(err.Error(), "already using bare-in-.git structure") {
		t.Errorf("error = %q, want it to contain 'already using bare-in-.git structure'", err.Error())
	}
}

func TestValidateMigration_WorktreePath(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	// Create a worktree — .git is a file, not a directory
	wtPath := filepath.Join(tmpDir, "wt-not-repo")
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "wt-branch", wtPath); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	_, err := ValidateMigration(ctx, wtPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test",
	})
	if err == nil {
		t.Fatal("expected error for worktree path")
	}
	if !strings.Contains(err.Error(), "worktree, not a repository") {
		t.Errorf("error = %q, want it to contain 'worktree, not a repository'", err.Error())
	}
}

func TestValidateMigration_NotAGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolveTempDir(t)
	_, err := ValidateMigration(context.Background(), tmpDir, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test",
	})
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error = %q, want it to contain 'not a git repository'", err.Error())
	}
}

func TestValidateMigration_Submodules(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Create .gitmodules file to simulate submodules
	if err := os.WriteFile(filepath.Join(repoPath, ".gitmodules"), []byte("[submodule \"sub\"]\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitmodules: %v", err)
	}

	_, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test",
	})
	if err == nil {
		t.Fatal("expected error for repo with submodules")
	}
	if !strings.Contains(err.Error(), "submodules are not yet supported") {
		t.Errorf("error = %q, want it to contain 'submodules are not yet supported'", err.Error())
	}
}

func TestValidateMigration_WorktreeFormatResolution(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Test with sibling format
	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "../{repo}-{branch}",
		RepoName:       "myrepo",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	// Sibling format: ../myrepo-main relative to repoPath
	parentDir := filepath.Dir(repoPath)
	wantPath := filepath.Join(parentDir, "myrepo-main")
	if plan.MainWorktreePath != wantPath {
		t.Errorf("MainWorktreePath = %q, want %q", plan.MainWorktreePath, wantPath)
	}
}

func TestValidateMigration_WithUpstream(t *testing.T) {
	t.Parallel()

	repoPath, _ := setupTestRepoWithOrigin(t)
	ctx := context.Background()

	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	// main has upstream tracking from setupTestRepoWithOrigin (push -u origin HEAD)
	if plan.MainBranchUpstream != "main" {
		t.Errorf("MainBranchUpstream = %q, want %q", plan.MainBranchUpstream, "main")
	}
}

func TestMigrateToBare_Simple(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Add more files so we can verify they survive migration
	if err := os.WriteFile(filepath.Join(repoPath, "file.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGit(ctx, repoPath, "add", "file.txt"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if err := runGit(ctx, repoPath, "commit", "-m", "Add file"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test-repo",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	result, err := MigrateToBare(ctx, plan)
	if err != nil {
		t.Fatalf("MigrateToBare failed: %v", err)
	}

	// Verify .git is now a bare repo
	gitDir := filepath.Join(repoPath, ".git")
	if !isBareRepo(gitDir) {
		t.Error(".git should be a bare repo after migration")
	}

	// Verify main worktree exists
	if _, err := os.Stat(result.MainWorktreePath); err != nil {
		t.Fatalf("main worktree should exist at %s: %v", result.MainWorktreePath, err)
	}

	// Verify files were moved to main worktree
	if _, err := os.Stat(filepath.Join(result.MainWorktreePath, "file.txt")); err != nil {
		t.Error("file.txt should exist in main worktree")
	}
	if _, err := os.Stat(filepath.Join(result.MainWorktreePath, "README.md")); err != nil {
		t.Error("README.md should exist in main worktree")
	}

	// Verify .git file in worktree points to correct metadata
	gitFile := filepath.Join(result.MainWorktreePath, ".git")
	data, err := os.ReadFile(gitFile)
	if err != nil {
		t.Fatalf("failed to read .git file in worktree: %v", err)
	}
	if !strings.HasPrefix(string(data), "gitdir:") {
		t.Errorf(".git file content = %q, want it to start with 'gitdir:'", string(data))
	}

	// Verify git status works in the new worktree
	if err := runGit(ctx, result.MainWorktreePath, "status"); err != nil {
		t.Errorf("git status failed in migrated worktree: %v", err)
	}

	// Verify branch is correct
	branch, err := GetCurrentBranch(ctx, result.MainWorktreePath)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "main" {
		t.Errorf("branch = %q, want main", branch)
	}
}

func TestMigrateToBare_WithExistingWorktrees(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	// Create a worktree outside the repo
	wtPath := filepath.Join(tmpDir, "wt-existing")
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "existing-branch", wtPath); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Add a file to the worktree
	if err := os.WriteFile(filepath.Join(wtPath, "wt-file.txt"), []byte("worktree\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGit(ctx, wtPath, "add", "wt-file.txt"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := runGit(ctx, wtPath, "commit", "-m", "Worktree commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test-repo",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	result, err := MigrateToBare(ctx, plan)
	if err != nil {
		t.Fatalf("MigrateToBare failed: %v", err)
	}

	// Verify main worktree works
	if err := runGit(ctx, result.MainWorktreePath, "status"); err != nil {
		t.Errorf("git status failed in main worktree: %v", err)
	}

	// Verify the existing worktree's branch still exists
	// The worktree was moved to match the format: {branch} → nested inside repo
	movedWT := filepath.Join(repoPath, "existing-branch")
	if _, err := os.Stat(movedWT); err != nil {
		t.Errorf("moved worktree should exist at %s: %v", movedWT, err)
	}

	// Verify git status works in the moved worktree
	if err := runGit(ctx, movedWT, "status"); err != nil {
		t.Errorf("git status failed in moved worktree: %v", err)
	}

	// Verify the branch is correct
	branch, err := GetCurrentBranch(ctx, movedWT)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "existing-branch" {
		t.Errorf("branch = %q, want existing-branch", branch)
	}

	// Verify committed file exists in the moved worktree
	if _, err := os.Stat(filepath.Join(movedWT, "wt-file.txt")); err != nil {
		t.Error("wt-file.txt should exist in moved worktree")
	}
}

func TestMigrateToBare_PreservesUpstream(t *testing.T) {
	t.Parallel()

	repoPath, _ := setupTestRepoWithOrigin(t)
	ctx := context.Background()

	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test-repo",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	result, err := MigrateToBare(ctx, plan)
	if err != nil {
		t.Fatalf("MigrateToBare failed: %v", err)
	}

	// Verify upstream was restored
	upstream := GetUpstreamBranch(ctx, result.MainWorktreePath, "main")
	if upstream != "main" {
		t.Errorf("upstream after migration = %q, want %q", upstream, "main")
	}
}

func TestMigrateToBare_SiblingFormat(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "../{repo}-{branch}",
		RepoName:       "myrepo",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	result, err := MigrateToBare(ctx, plan)
	if err != nil {
		t.Fatalf("MigrateToBare failed: %v", err)
	}

	// Verify main worktree is a sibling
	parentDir := filepath.Dir(repoPath)
	wantPath := filepath.Join(parentDir, "myrepo-main")
	if result.MainWorktreePath != wantPath {
		t.Errorf("MainWorktreePath = %q, want %q", result.MainWorktreePath, wantPath)
	}

	// Verify worktree is functional
	if err := runGit(ctx, result.MainWorktreePath, "status"); err != nil {
		t.Errorf("git status failed in sibling worktree: %v", err)
	}

	// Verify files are in the sibling worktree
	if _, err := os.Stat(filepath.Join(result.MainWorktreePath, "README.md")); err != nil {
		t.Error("README.md should exist in sibling worktree")
	}
}

func TestMigrateToBare_PreservesUncommittedChanges(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Create a staged-but-uncommitted file
	if err := os.WriteFile(filepath.Join(repoPath, "staged.txt"), []byte("staged\n"), 0644); err != nil {
		t.Fatalf("failed to write staged.txt: %v", err)
	}
	if err := runGit(ctx, repoPath, "add", "staged.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	// Create a modified-but-unstaged file (modify the existing README.md)
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# modified\n"), 0644); err != nil {
		t.Fatalf("failed to modify README.md: %v", err)
	}

	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test-repo",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	result, err := MigrateToBare(ctx, plan)
	if err != nil {
		t.Fatalf("MigrateToBare failed: %v", err)
	}

	// Verify staged file exists in worktree
	if _, err := os.Stat(filepath.Join(result.MainWorktreePath, "staged.txt")); err != nil {
		t.Error("staged.txt should exist in main worktree")
	}

	// Verify modified file has the new content
	data, err := os.ReadFile(filepath.Join(result.MainWorktreePath, "README.md"))
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}
	if string(data) != "# modified\n" {
		t.Errorf("README.md content = %q, want %q", string(data), "# modified\n")
	}

	// Verify git status shows staged file as added and README.md as modified
	statusOut, err := outputGit(ctx, result.MainWorktreePath, "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	status := strings.TrimSpace(string(statusOut))

	if !strings.Contains(status, "staged.txt") {
		t.Errorf("expected staged.txt in git status, got:\n%s", status)
	}
	if !strings.Contains(status, "README.md") {
		t.Errorf("expected README.md in git status, got:\n%s", status)
	}
}

func TestMigrateToBare_BranchWithSlashes(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	// Create a worktree with a branch containing slashes
	wtPath := filepath.Join(tmpDir, "wt-feature")
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "feature/my-branch", wtPath); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	plan, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test-repo",
	})
	if err != nil {
		t.Fatalf("ValidateMigration failed: %v", err)
	}

	// Verify the plan sanitizes the branch name
	if len(plan.WorktreesToFix) != 1 {
		t.Fatalf("WorktreesToFix = %d, want 1", len(plan.WorktreesToFix))
	}
	wt := plan.WorktreesToFix[0]
	if wt.NewName != "feature-my-branch" {
		t.Errorf("NewName = %q, want %q", wt.NewName, "feature-my-branch")
	}
	if wt.OldName == wt.NewName {
		t.Error("OldName should differ from NewName for slash-containing branch")
	}

	result, err := MigrateToBare(ctx, plan)
	if err != nil {
		t.Fatalf("MigrateToBare failed: %v", err)
	}

	// Verify the worktree was moved to the sanitized path
	movedWT := filepath.Join(repoPath, "feature-my-branch")
	if _, err := os.Stat(movedWT); err != nil {
		t.Fatalf("moved worktree should exist at %s: %v", movedWT, err)
	}

	// Verify git status works in the moved worktree
	if err := runGit(ctx, movedWT, "status"); err != nil {
		t.Errorf("git status failed in moved worktree: %v", err)
	}

	// Verify the branch is correct
	branch, err := GetCurrentBranch(ctx, movedWT)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "feature/my-branch" {
		t.Errorf("branch = %q, want feature/my-branch", branch)
	}

	// Verify metadata directory uses sanitized name
	metaDir := filepath.Join(repoPath, ".git", "worktrees", "feature-my-branch")
	if _, err := os.Stat(metaDir); err != nil {
		t.Errorf("metadata dir should exist at %s: %v", metaDir, err)
	}

	// Verify main worktree also works
	if err := runGit(ctx, result.MainWorktreePath, "status"); err != nil {
		t.Errorf("git status failed in main worktree: %v", err)
	}
}

func TestValidateMigration_TargetPathConflict(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	// Create a worktree at a location that won't match the format
	wtPath := filepath.Join(tmpDir, "old-location")
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "conflict-branch", wtPath); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Pre-create the target path that the worktree would be moved to
	conflictPath := filepath.Join(repoPath, "conflict-branch")
	if err := os.MkdirAll(conflictPath, 0755); err != nil {
		t.Fatalf("failed to create conflict dir: %v", err)
	}

	_, err := ValidateMigration(ctx, repoPath, MigrationOptions{
		WorktreeFormat: "{branch}",
		RepoName:       "test-repo",
	})
	if err == nil {
		t.Fatal("expected error for target path conflict")
	}
	if !strings.Contains(err.Error(), "target path conflict") {
		t.Errorf("error = %q, want it to contain 'target path conflict'", err.Error())
	}
}
