package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeProjectPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple absolute path",
			path: "/home/user/myproject",
			want: "-home-user-myproject",
		},
		{
			name: "path with trailing slash",
			path: "/home/user/myproject/",
			want: "-home-user-myproject",
		},
		{
			name: "macOS-style path",
			path: "/Users/sean/myproject",
			want: "-Users-sean-myproject",
		},
		{
			name: "deeply nested path",
			path: "/home/user/repos/org/myproject",
			want: "-home-user-repos-org-myproject",
		},
		{
			name: "path with hyphens",
			path: "/home/user/my-project",
			want: "-home-user-my-project",
		},
		{
			name: "root path",
			path: "/",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := EncodeProjectPath(tt.path)
			if got != tt.want {
				t.Errorf("EncodeProjectPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSymlinkProjectDir(t *testing.T) {
	t.Parallel()

	// Set up a fake Claude config dir
	tmpDir := resolvePath(t, t.TempDir())
	claudeDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	mainRepo := filepath.Join(tmpDir, "repos", "myapp")
	worktree := filepath.Join(tmpDir, "repos", "myapp-feature")

	// Create directories
	if err := os.MkdirAll(mainRepo, 0755); err != nil {
		t.Fatalf("create main repo dir: %v", err)
	}
	if err := os.MkdirAll(worktree, 0755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}

	// Run symlink
	if err := SymlinkProjectDir(mainRepo, worktree); err != nil {
		t.Fatalf("SymlinkProjectDir() error = %v", err)
	}

	// Verify the symlink was created
	projectsDir := filepath.Join(claudeDir, "projects")
	wtEncoded := EncodeProjectPath(worktree)
	mainEncoded := EncodeProjectPath(mainRepo)

	symlinkPath := filepath.Join(projectsDir, wtEncoded)
	targetPath := filepath.Join(projectsDir, mainEncoded)

	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("Lstat symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink, got regular file/dir")
	}

	// Verify target
	link, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if link != targetPath {
		t.Errorf("symlink target = %q, want %q", link, targetPath)
	}
}

func TestSymlinkProjectDir_Idempotent(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	claudeDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	mainRepo := filepath.Join(tmpDir, "repos", "myapp")
	worktree := filepath.Join(tmpDir, "repos", "myapp-feature")

	if err := os.MkdirAll(mainRepo, 0755); err != nil {
		t.Fatalf("create main repo dir: %v", err)
	}
	if err := os.MkdirAll(worktree, 0755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}

	// Run twice — should not error
	if err := SymlinkProjectDir(mainRepo, worktree); err != nil {
		t.Fatalf("first SymlinkProjectDir() error = %v", err)
	}
	if err := SymlinkProjectDir(mainRepo, worktree); err != nil {
		t.Fatalf("second SymlinkProjectDir() error = %v", err)
	}
}

func TestSymlinkProjectDir_SkipSamePath(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	claudeDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	repoPath := filepath.Join(tmpDir, "repos", "myapp")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}

	// Same path should be a no-op
	if err := SymlinkProjectDir(repoPath, repoPath); err != nil {
		t.Fatalf("SymlinkProjectDir(same, same) error = %v", err)
	}

	// No symlink should have been created
	projectsDir := filepath.Join(claudeDir, "projects")
	_, err := os.Stat(projectsDir)
	if !os.IsNotExist(err) {
		t.Errorf("expected projects dir to not exist when paths are same, got err=%v", err)
	}
}

func TestSymlinkProjectDir_ExistingDirUntouched(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	claudeDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	mainRepo := filepath.Join(tmpDir, "repos", "myapp")
	worktree := filepath.Join(tmpDir, "repos", "myapp-feature")

	if err := os.MkdirAll(mainRepo, 0755); err != nil {
		t.Fatalf("create main repo dir: %v", err)
	}
	if err := os.MkdirAll(worktree, 0755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}

	// Pre-create the worktree project dir as a real directory with data
	wtProjectDir := filepath.Join(claudeDir, "projects", EncodeProjectPath(worktree))
	if err := os.MkdirAll(wtProjectDir, 0755); err != nil {
		t.Fatalf("create wt project dir: %v", err)
	}
	sentinel := filepath.Join(wtProjectDir, "session.jsonl")
	if err := os.WriteFile(sentinel, []byte("data"), 0644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	// Symlink should not overwrite existing directory
	if err := SymlinkProjectDir(mainRepo, worktree); err != nil {
		t.Fatalf("SymlinkProjectDir() error = %v", err)
	}

	// Verify the directory was NOT replaced
	info, err := os.Lstat(wtProjectDir)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("expected regular directory to be preserved, got symlink")
	}

	// Sentinel file should still exist
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel file lost: %v", err)
	}
}

func TestSymlinkProjectDir_WrongSymlinkReplaced(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	claudeDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	mainRepo := filepath.Join(tmpDir, "repos", "myapp")
	oldRepo := filepath.Join(tmpDir, "repos", "old-repo")
	worktree := filepath.Join(tmpDir, "repos", "myapp-feature")

	for _, dir := range []string{mainRepo, oldRepo, worktree} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create dir %s: %v", dir, err)
		}
	}

	projectsDir := filepath.Join(claudeDir, "projects")
	wtEncoded := EncodeProjectPath(worktree)
	mainEncoded := EncodeProjectPath(mainRepo)
	oldEncoded := EncodeProjectPath(oldRepo)

	// Create the old target dir and a wrong symlink
	oldTarget := filepath.Join(projectsDir, oldEncoded)
	if err := os.MkdirAll(oldTarget, 0755); err != nil {
		t.Fatalf("create old target: %v", err)
	}
	symlinkPath := filepath.Join(projectsDir, wtEncoded)
	if err := os.Symlink(oldTarget, symlinkPath); err != nil {
		t.Fatalf("create old symlink: %v", err)
	}

	// Now create the correct symlink
	if err := SymlinkProjectDir(mainRepo, worktree); err != nil {
		t.Fatalf("SymlinkProjectDir() error = %v", err)
	}

	// Verify it now points to the correct target
	link, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	expectedTarget := filepath.Join(projectsDir, mainEncoded)
	if link != expectedTarget {
		t.Errorf("symlink target = %q, want %q", link, expectedTarget)
	}
}

func TestRemoveProjectDirSymlink(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	claudeDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	mainRepo := filepath.Join(tmpDir, "repos", "myapp")
	worktree := filepath.Join(tmpDir, "repos", "myapp-feature")

	if err := os.MkdirAll(mainRepo, 0755); err != nil {
		t.Fatalf("create main repo dir: %v", err)
	}
	if err := os.MkdirAll(worktree, 0755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}

	// Create symlink
	if err := SymlinkProjectDir(mainRepo, worktree); err != nil {
		t.Fatalf("SymlinkProjectDir() error = %v", err)
	}

	// Verify symlink exists
	symlinkPath := filepath.Join(claudeDir, "projects", EncodeProjectPath(worktree))
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink before removal")
	}

	// Remove symlink
	if err := RemoveProjectDirSymlink(worktree); err != nil {
		t.Fatalf("RemoveProjectDirSymlink() error = %v", err)
	}

	// Verify symlink is gone
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Errorf("expected symlink to be removed, got err=%v", err)
	}
}

func TestRemoveProjectDirSymlink_NonexistentIsNoOp(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	claudeDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	// Should not error when nothing exists
	if err := RemoveProjectDirSymlink(filepath.Join(tmpDir, "nonexistent")); err != nil {
		t.Fatalf("RemoveProjectDirSymlink() error = %v", err)
	}
}

func TestRemoveProjectDirSymlink_RegularDirUntouched(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	claudeDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	worktree := filepath.Join(tmpDir, "repos", "myapp-feature")
	if err := os.MkdirAll(worktree, 0755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}

	// Create real directory (not symlink)
	wtProjectDir := filepath.Join(claudeDir, "projects", EncodeProjectPath(worktree))
	if err := os.MkdirAll(wtProjectDir, 0755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	// Should not remove regular directories
	if err := RemoveProjectDirSymlink(worktree); err != nil {
		t.Fatalf("RemoveProjectDirSymlink() error = %v", err)
	}

	// Directory should still exist
	if _, err := os.Stat(wtProjectDir); err != nil {
		t.Errorf("regular directory was removed: %v", err)
	}
}

func TestGetConfigDir_EnvOverride(t *testing.T) {
	t.Parallel()

	t.Setenv("CLAUDE_CONFIG_DIR", "/custom/claude/dir")

	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error = %v", err)
	}
	if dir != "/custom/claude/dir" {
		t.Errorf("GetConfigDir() = %q, want /custom/claude/dir", dir)
	}
}

func TestGetConfigDir_DefaultsToHome(t *testing.T) {
	t.Parallel()

	t.Setenv("CLAUDE_CONFIG_DIR", "")

	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".claude")
	if dir != want {
		t.Errorf("GetConfigDir() = %q, want %q", dir, want)
	}
}

// resolvePath resolves symlinks in paths for macOS compatibility
// where t.TempDir() may return /tmp which symlinks to /private/tmp.
func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve path %s: %v", path, err)
	}
	return resolved
}
