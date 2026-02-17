package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// resolveTempDir creates a temp directory and resolves macOS symlinks.
func resolveTempDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for %s: %v", tmpDir, err)
	}
	return resolved
}

// configureTestRepo sets git user config and disables GPG signing.
func configureTestRepo(t *testing.T, repoPath string) {
	t.Helper()
	ctx := context.Background()
	for _, args := range [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	} {
		if err := runGit(ctx, repoPath, args...); err != nil {
			t.Fatalf("failed to run git %v: %v", args, err)
		}
	}
}

// setupTestRepo creates a git repo with main branch, initial commit, and git config.
// Returns the resolved repo path.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := resolveTempDir(t)
	repoPath := filepath.Join(tmpDir, "test-repo")

	ctx := context.Background()
	if err := runGit(ctx, "", "init", "-b", "main", repoPath); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	configureTestRepo(t, repoPath)

	// Create initial commit
	readme := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGit(ctx, repoPath, "add", "README.md"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if err := runGit(ctx, repoPath, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	return repoPath
}

// assertContains checks that all wanted items exist in the got slice.
func assertContains(t *testing.T, got []string, want ...string) {
	t.Helper()
	set := make(map[string]bool, len(got))
	for _, s := range got {
		set[s] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("missing %q in %v", w, got)
		}
	}
}

// setupTestRepoWithOrigin creates a repo with a bare origin remote.
// Returns (repoPath, originPath).
func setupTestRepoWithOrigin(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := resolveTempDir(t)

	originPath := filepath.Join(tmpDir, "origin.git")
	repoPath := filepath.Join(tmpDir, "repo")

	ctx := context.Background()

	// Create bare origin (-b main ensures consistent default branch across git versions)
	if err := runGit(ctx, "", "init", "--bare", "-b", "main", originPath); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	// Clone from bare origin
	if err := runGit(ctx, "", "clone", originPath, repoPath); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	configureTestRepo(t, repoPath)

	// Create initial commit and push
	readme := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGit(ctx, repoPath, "add", "README.md"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := runGit(ctx, repoPath, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if err := runGit(ctx, repoPath, "push", "-u", "origin", "HEAD"); err != nil {
		t.Fatalf("failed to push: %v", err)
	}

	return repoPath, originPath
}

func TestGetMainRepoPath(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	wtPath := filepath.Join(tmpDir, "test-worktree")

	ctx := context.Background()
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "test-branch", wtPath); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	mainPath, err := GetMainRepoPath(wtPath)
	if err != nil {
		t.Errorf("GetMainRepoPath from worktree failed: %v", err)
	}
	if mainPath != repoPath {
		t.Errorf("expected %s, got %s", repoPath, mainPath)
	}

	mainPathFromRepo, err := GetMainRepoPath(repoPath)
	if err != nil {
		t.Errorf("GetMainRepoPath from main repo failed: %v", err)
	}
	if mainPathFromRepo != repoPath {
		t.Errorf("expected %s, got %s", repoPath, mainPathFromRepo)
	}

	emptyDir := t.TempDir()
	_, err = GetMainRepoPath(emptyDir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestGetDefaultBranch(t *testing.T) {
	t.Parallel()

	result := GetDefaultBranch(context.Background(), "/nonexistent/path")
	if result != "main" && result != "master" {
		t.Errorf("expected main or master as fallback, got %s", result)
	}
}

func TestWorktreeStruct(t *testing.T) {
	t.Parallel()

	wt := Worktree{
		Path:     "/test/path",
		Branch:   "feature-branch",
		RepoPath: "/test/main",
		RepoName: "test-repo",
		PRState:  "MERGED",
	}

	if wt.Path != "/test/path" {
		t.Errorf("unexpected path: %s", wt.Path)
	}
	if wt.Branch != "feature-branch" {
		t.Errorf("unexpected branch: %s", wt.Branch)
	}
	if wt.PRState != "MERGED" {
		t.Errorf("expected PRState to be MERGED, got %s", wt.PRState)
	}
}

func TestCreateWorktreeResult(t *testing.T) {
	t.Parallel()

	result := &CreateWorktreeResult{
		Path:          "/test/worktree",
		AlreadyExists: true,
	}

	if result.Path != "/test/worktree" {
		t.Errorf("unexpected path: %s", result.Path)
	}
	if !result.AlreadyExists {
		t.Error("expected AlreadyExists to be true")
	}
}

func TestParseRemoteRef(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	if err := runGit(ctx, repoPath, "remote", "add", "origin", "https://github.com/test/repo.git"); err != nil {
		t.Fatalf("failed to add origin: %v", err)
	}
	if err := runGit(ctx, repoPath, "remote", "add", "upstream", "https://github.com/upstream/repo.git"); err != nil {
		t.Fatalf("failed to add upstream: %v", err)
	}

	tests := []struct {
		ref          string
		wantRemote   string
		wantBranch   string
		wantIsRemote bool
	}{
		{"main", "", "main", false},
		{"feature/test", "", "feature/test", false},
		{"origin/main", "origin", "main", true},
		{"origin/feature/test", "origin", "feature/test", true},
		{"upstream/develop", "upstream", "develop", true},
		{"nonexistent/branch", "", "nonexistent/branch", false},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			t.Parallel()
			remote, branch, isRemote := ParseRemoteRef(ctx, repoPath, tt.ref)
			if remote != tt.wantRemote {
				t.Errorf("remote: got %q, want %q", remote, tt.wantRemote)
			}
			if branch != tt.wantBranch {
				t.Errorf("branch: got %q, want %q", branch, tt.wantBranch)
			}
			if isRemote != tt.wantIsRemote {
				t.Errorf("isRemote: got %v, want %v", isRemote, tt.wantIsRemote)
			}
		})
	}
}

func TestGetRepoDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{"/home/user/repos/my-project", "my-project"},
		{"/tmp/repo", "repo"},
		{"repo", "repo"},
		{"/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := GetRepoDisplayName(tt.path)
			if got != tt.want {
				t.Errorf("GetRepoDisplayName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectRepoType(t *testing.T) {
	t.Parallel()

	t.Run("regular repo", func(t *testing.T) {
		t.Parallel()
		repoPath := setupTestRepo(t)

		repoType, err := DetectRepoType(repoPath)
		if err != nil {
			t.Fatalf("DetectRepoType failed: %v", err)
		}
		if repoType != RepoTypeRegular {
			t.Errorf("expected RepoTypeRegular, got %v", repoType)
		}
	})

	t.Run("bare repo", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		resolved, err := filepath.EvalSymlinks(tmpDir)
		if err != nil {
			t.Fatalf("failed to resolve symlinks: %v", err)
		}
		barePath := filepath.Join(resolved, "bare.git")

		ctx := context.Background()
		if err := runGit(ctx, "", "init", "--bare", barePath); err != nil {
			t.Fatalf("failed to init bare repo: %v", err)
		}

		repoType, err := DetectRepoType(barePath)
		if err != nil {
			t.Fatalf("DetectRepoType failed: %v", err)
		}
		if repoType != RepoTypeBare {
			t.Errorf("expected RepoTypeBare, got %v", repoType)
		}
	})

	t.Run("non-git dir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		_, err := DetectRepoType(tmpDir)
		if err == nil {
			t.Error("expected error for non-git directory")
		}
	})
}

func TestGetGitDir(t *testing.T) {
	t.Parallel()

	t.Run("regular repo", func(t *testing.T) {
		t.Parallel()
		repoPath := setupTestRepo(t)

		gitDir := GetGitDir(repoPath, RepoTypeRegular)
		want := filepath.Join(repoPath, ".git")
		if gitDir != want {
			t.Errorf("GetGitDir = %q, want %q", gitDir, want)
		}
	})

	t.Run("bare repo", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		resolved, err := filepath.EvalSymlinks(tmpDir)
		if err != nil {
			t.Fatalf("failed to resolve symlinks: %v", err)
		}
		barePath := filepath.Join(resolved, "bare.git")

		ctx := context.Background()
		if err := runGit(ctx, "", "init", "--bare", barePath); err != nil {
			t.Fatalf("failed to init bare repo: %v", err)
		}

		gitDir := GetGitDir(barePath, RepoTypeBare)
		if gitDir != barePath {
			t.Errorf("GetGitDir = %q, want %q", gitDir, barePath)
		}
	})
}

func TestGetCurrentBranch(t *testing.T) {
	t.Parallel()

	t.Run("on main", func(t *testing.T) {
		t.Parallel()
		repoPath := setupTestRepo(t)
		ctx := context.Background()

		branch, err := GetCurrentBranch(ctx, repoPath)
		if err != nil {
			t.Fatalf("GetCurrentBranch failed: %v", err)
		}
		if branch != "main" {
			t.Errorf("branch = %q, want main", branch)
		}
	})

	t.Run("on feature branch", func(t *testing.T) {
		t.Parallel()
		repoPath := setupTestRepo(t)
		ctx := context.Background()

		if err := runGit(ctx, repoPath, "checkout", "-b", "feature-x"); err != nil {
			t.Fatalf("failed to create branch: %v", err)
		}
		branch, err := GetCurrentBranch(ctx, repoPath)
		if err != nil {
			t.Fatalf("GetCurrentBranch failed: %v", err)
		}
		if branch != "feature-x" {
			t.Errorf("branch = %q, want feature-x", branch)
		}
	})
}

func TestListWorktreesFromRepo(t *testing.T) {
	t.Parallel()

	t.Run("no extra worktrees", func(t *testing.T) {
		t.Parallel()
		repoPath := setupTestRepo(t)
		ctx := context.Background()

		wts, err := ListWorktreesFromRepo(ctx, repoPath)
		if err != nil {
			t.Fatalf("ListWorktreesFromRepo failed: %v", err)
		}
		// Only the main worktree
		if len(wts) != 1 {
			t.Errorf("got %d worktrees, want 1 (main only)", len(wts))
		}
		if len(wts) > 0 && wts[0].Branch != "main" {
			t.Errorf("main worktree branch = %q, want main", wts[0].Branch)
		}
	})

	t.Run("with extra worktrees", func(t *testing.T) {
		t.Parallel()
		repoPath := setupTestRepo(t)
		tmpDir := filepath.Dir(repoPath)
		ctx := context.Background()

		wt1 := filepath.Join(tmpDir, "wt-feature-1")
		wt2 := filepath.Join(tmpDir, "wt-feature-2")

		if err := runGit(ctx, repoPath, "worktree", "add", "-b", "feature-1", wt1); err != nil {
			t.Fatalf("failed to create worktree 1: %v", err)
		}
		if err := runGit(ctx, repoPath, "worktree", "add", "-b", "feature-2", wt2); err != nil {
			t.Fatalf("failed to create worktree 2: %v", err)
		}

		wts, err := ListWorktreesFromRepo(ctx, repoPath)
		if err != nil {
			t.Fatalf("ListWorktreesFromRepo failed: %v", err)
		}
		// main + 2 worktrees
		if len(wts) != 3 {
			t.Errorf("got %d worktrees, want 3", len(wts))
		}

		var branches []string
		for _, wt := range wts {
			branches = append(branches, wt.Branch)
		}
		assertContains(t, branches, "main", "feature-1", "feature-2")
	})
}

func TestGetAllBranchConfig(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Create branches and set descriptions/upstreams
	if err := runGit(ctx, repoPath, "branch", "feature-a"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := runGit(ctx, repoPath, "branch", "feature-b"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	if err := runGit(ctx, repoPath, "config", "branch.feature-a.description", "Working on A"); err != nil {
		t.Fatalf("failed to set description: %v", err)
	}
	if err := runGit(ctx, repoPath, "config", "branch.feature-b.merge", "refs/heads/feature-b"); err != nil {
		t.Fatalf("failed to set merge: %v", err)
	}

	notes, upstreams := GetAllBranchConfig(ctx, repoPath)

	if notes["feature-a"] != "Working on A" {
		t.Errorf("notes[feature-a] = %q, want %q", notes["feature-a"], "Working on A")
	}
	if !upstreams["feature-b"] {
		t.Error("upstreams[feature-b] should be true")
	}
	if upstreams["feature-a"] {
		t.Error("upstreams[feature-a] should be false")
	}
}

func TestListLocalBranches(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	if err := runGit(ctx, repoPath, "branch", "alpha"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := runGit(ctx, repoPath, "branch", "beta"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	branches, err := ListLocalBranches(ctx, repoPath)
	if err != nil {
		t.Fatalf("ListLocalBranches failed: %v", err)
	}

	assertContains(t, branches, "main", "alpha", "beta")
}

func TestListRemoteBranches(t *testing.T) {
	t.Parallel()

	repoPath, _ := setupTestRepoWithOrigin(t)
	ctx := context.Background()

	// Create and push a feature branch
	if err := runGit(ctx, repoPath, "checkout", "-b", "feature-remote"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := runGit(ctx, repoPath, "push", "-u", "origin", "feature-remote"); err != nil {
		t.Fatalf("failed to push: %v", err)
	}

	branches, err := ListRemoteBranches(ctx, repoPath)
	if err != nil {
		t.Fatalf("ListRemoteBranches failed: %v", err)
	}

	assertContains(t, branches, "origin/feature-remote")
}

func TestBranchExists(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	if err := runGit(ctx, repoPath, "branch", "existing"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	t.Run("LocalBranchExists", func(t *testing.T) {
		t.Parallel()
		if !LocalBranchExists(ctx, repoPath, "existing") {
			t.Error("existing branch should exist")
		}
		if LocalBranchExists(ctx, repoPath, "nonexistent") {
			t.Error("nonexistent branch should not exist")
		}
	})

	t.Run("RefExists", func(t *testing.T) {
		t.Parallel()
		if !RefExists(ctx, repoPath, "HEAD") {
			t.Error("HEAD should exist")
		}
		if !RefExists(ctx, repoPath, "refs/heads/main") {
			t.Error("refs/heads/main should exist")
		}
		if RefExists(ctx, repoPath, "refs/heads/nonexistent") {
			t.Error("nonexistent ref should not exist")
		}
	})
}

func TestRemoteBranchExists(t *testing.T) {
	t.Parallel()

	repoPath, _ := setupTestRepoWithOrigin(t)
	ctx := context.Background()

	// setupTestRepoWithOrigin pushes to origin, creating origin/main
	if !RemoteBranchExists(ctx, repoPath, "main") {
		t.Error("remote branch \"main\" should exist")
	}
	if RemoteBranchExists(ctx, repoPath, "nonexistent-remote") {
		t.Error("nonexistent remote branch should not exist")
	}
}

func TestListRemotes(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	if err := runGit(ctx, repoPath, "remote", "add", "origin", "https://github.com/test/repo.git"); err != nil {
		t.Fatalf("failed to add origin: %v", err)
	}
	if err := runGit(ctx, repoPath, "remote", "add", "upstream", "https://github.com/upstream/repo.git"); err != nil {
		t.Fatalf("failed to add upstream: %v", err)
	}

	remotes, err := ListRemotes(ctx, repoPath)
	if err != nil {
		t.Fatalf("ListRemotes failed: %v", err)
	}

	assertContains(t, remotes, "origin", "upstream")
}

func TestUpstreamBranch(t *testing.T) {
	t.Parallel()

	repoPath, _ := setupTestRepoWithOrigin(t)
	ctx := context.Background()

	// Create a feature branch
	if err := runGit(ctx, repoPath, "checkout", "-b", "feature-up"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := runGit(ctx, repoPath, "push", "-u", "origin", "feature-up"); err != nil {
		t.Fatalf("failed to push: %v", err)
	}

	// Get upstream
	upstream := GetUpstreamBranch(ctx, repoPath, "feature-up")
	if upstream != "feature-up" {
		t.Errorf("GetUpstreamBranch = %q, want %q", upstream, "feature-up")
	}

	// No upstream configured
	if err := runGit(ctx, repoPath, "checkout", "-b", "no-upstream"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	upstream = GetUpstreamBranch(ctx, repoPath, "no-upstream")
	if upstream != "" {
		t.Errorf("GetUpstreamBranch for no-upstream = %q, want empty", upstream)
	}

	// Set upstream and verify
	if err := SetUpstreamBranch(ctx, repoPath, "no-upstream", "feature-up"); err != nil {
		t.Fatalf("SetUpstreamBranch failed: %v", err)
	}
	upstream = GetUpstreamBranch(ctx, repoPath, "no-upstream")
	if upstream != "feature-up" {
		t.Errorf("GetUpstreamBranch after set = %q, want %q", upstream, "feature-up")
	}
}

func TestGetCurrentRepoMainPathFrom(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	// Create a worktree
	wtPath := filepath.Join(tmpDir, "wt-main-path-test")
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "main-path-branch", wtPath); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// From worktree should return main repo
	got := GetCurrentRepoMainPathFrom(ctx, wtPath)
	if got != repoPath {
		t.Errorf("from worktree: got %q, want %q", got, repoPath)
	}

	// From main repo should return itself
	got = GetCurrentRepoMainPathFrom(ctx, repoPath)
	if got != repoPath {
		t.Errorf("from main repo: got %q, want %q", got, repoPath)
	}

	// From non-git dir should return empty string
	nonGitDir := t.TempDir()
	got = GetCurrentRepoMainPathFrom(ctx, nonGitDir)
	if got != "" {
		t.Errorf("from non-git dir: got %q, want empty", got)
	}
}
