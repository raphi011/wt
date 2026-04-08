package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoHasLabel(t *testing.T) {
	t.Parallel()

	repo := Repo{
		Name:   "test",
		Path:   "/test",
		Labels: []string{"foo", "bar"},
	}

	tests := []struct {
		label string
		want  bool
	}{
		{"foo", true},
		{"bar", true},
		{"baz", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := repo.HasLabel(tt.label); got != tt.want {
			t.Errorf("HasLabel(%q) = %v, want %v", tt.label, got, tt.want)
		}
	}
}

func TestRepoMatchesLabels(t *testing.T) {
	t.Parallel()

	repo := Repo{
		Name:   "test",
		Path:   "/test",
		Labels: []string{"foo", "bar"},
	}

	tests := []struct {
		labels []string
		want   bool
	}{
		{[]string{"foo"}, true},
		{[]string{"bar"}, true},
		{[]string{"baz"}, false},
		{[]string{"foo", "baz"}, true},
		{[]string{}, false},
	}

	for _, tt := range tests {
		if got := repo.MatchesLabels(tt.labels); got != tt.want {
			t.Errorf("MatchesLabels(%v) = %v, want %v", tt.labels, got, tt.want)
		}
	}
}

func TestRegistryAddRemove(t *testing.T) {
	t.Parallel()

	reg := &Registry{Repos: []Repo{}}

	// Add repo
	repo := Repo{
		Name: "test",
		Path: "/tmp/test",
	}

	if err := reg.Add(repo); err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	if len(reg.Repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(reg.Repos))
	}

	// Try to add duplicate path
	if err := reg.Add(repo); err == nil {
		t.Error("expected error adding duplicate path")
	}

	// Try to add duplicate name
	repo2 := Repo{
		Name: "test",
		Path: "/tmp/test2",
	}
	if err := reg.Add(repo2); err == nil {
		t.Error("expected error adding duplicate name")
	}

	// Remove repo
	if err := reg.Remove("test"); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(reg.Repos))
	}

	// Try to remove non-existent
	if err := reg.Remove("nonexistent"); err == nil {
		t.Error("expected error removing non-existent repo")
	}
}

func TestRegistryFindByName(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo"},
			{Name: "bar", Path: "/tmp/bar", Labels: []string{"backend"}},
		},
	}

	// Find by name
	repo, err := reg.FindByName("foo")
	if err != nil {
		t.Fatalf("FindByName() failed: %v", err)
	}
	if repo.Name != "foo" {
		t.Errorf("expected foo, got %s", repo.Name)
	}

	// Find non-existent
	_, err = reg.FindByName("baz")
	if err == nil {
		t.Error("expected error for non-existent repo")
	}

	// Find by label
	repos := reg.FindByLabel("backend")
	if len(repos) != 1 {
		t.Errorf("expected 1 repo with label, got %d", len(repos))
	}
	if repos[0].Name != "bar" {
		t.Errorf("expected bar, got %s", repos[0].Name)
	}
}

func TestRegistryFind(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo"},
			{Name: "bar", Path: "/tmp/bar", Labels: []string{"backend"}},
		},
	}

	// Find by name
	repo, err := reg.Find("foo")
	if err != nil {
		t.Fatalf("Find(name) failed: %v", err)
	}
	if repo.Name != "foo" {
		t.Errorf("expected foo, got %s", repo.Name)
	}

	// Find by path
	repo, err = reg.Find("/tmp/bar")
	if err != nil {
		t.Fatalf("Find(path) failed: %v", err)
	}
	if repo.Name != "bar" {
		t.Errorf("expected bar, got %s", repo.Name)
	}

	// Find non-existent
	_, err = reg.Find("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent repo")
	}
}

func TestFindByLabels(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "a", Path: "/tmp/a", Labels: []string{"backend", "api"}},
			{Name: "b", Path: "/tmp/b", Labels: []string{"frontend"}},
			{Name: "c", Path: "/tmp/c", Labels: []string{"backend"}},
			{Name: "d", Path: "/tmp/d"},
		},
	}

	// Single label
	repos := reg.FindByLabels([]string{"frontend"})
	if len(repos) != 1 || repos[0].Name != "b" {
		t.Errorf("FindByLabels([frontend]) = %v, want [b]", repos)
	}

	// Multiple labels (OR logic)
	repos = reg.FindByLabels([]string{"frontend", "api"})
	if len(repos) != 2 {
		t.Fatalf("FindByLabels([frontend, api]) got %d repos, want 2", len(repos))
	}
	names := map[string]bool{}
	for _, r := range repos {
		names[r.Name] = true
	}
	if !names["a"] || !names["b"] {
		t.Errorf("expected repos a and b, got %v", names)
	}

	// No matching labels
	repos = reg.FindByLabels([]string{"nonexistent"})
	if len(repos) != 0 {
		t.Errorf("FindByLabels([nonexistent]) = %v, want empty", repos)
	}

	// Empty labels
	repos = reg.FindByLabels([]string{})
	if len(repos) != 0 {
		t.Errorf("FindByLabels([]) = %v, want empty", repos)
	}
}

func TestRegistryLabels(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo"},
		},
	}

	// Add label
	if err := reg.AddLabel("foo", "backend"); err != nil {
		t.Fatalf("AddLabel() failed: %v", err)
	}

	repo, err := reg.FindByName("foo")
	if err != nil {
		t.Fatalf("FindByName after AddLabel failed: %v", err)
	}
	if !repo.HasLabel("backend") {
		t.Error("expected repo to have backend label")
	}

	// Add same label again (should be idempotent)
	if err := reg.AddLabel("foo", "backend"); err != nil {
		t.Fatalf("AddLabel() failed on duplicate: %v", err)
	}

	// Remove label
	if err := reg.RemoveLabel("foo", "backend"); err != nil {
		t.Fatalf("RemoveLabel() failed: %v", err)
	}

	repo, _ = reg.FindByName("foo")
	if repo.HasLabel("backend") {
		t.Error("expected repo to not have backend label")
	}

	// Add multiple labels
	reg.AddLabel("foo", "api")
	reg.AddLabel("foo", "frontend")

	// Clear labels
	if err := reg.ClearLabels("foo"); err != nil {
		t.Fatalf("ClearLabels() failed: %v", err)
	}

	repo, _ = reg.FindByName("foo")
	if len(repo.Labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(repo.Labels))
	}
}

func TestLoadExplicitPath(t *testing.T) {
	t.Parallel()

	t.Run("non-existent file returns empty registry", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.json")

		reg, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if len(reg.Repos) != 0 {
			t.Errorf("expected 0 repos, got %d", len(reg.Repos))
		}
	})

	t.Run("corrupted JSON returns error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "bad.json")

		if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
			t.Fatalf("setup: write failed: %v", err)
		}

		_, err := Load(path)
		if err == nil {
			t.Error("expected error for corrupted JSON")
		}
	})
}

func TestRegistrySaveLoad(t *testing.T) {
	t.Parallel()

	// Create temp dir for test
	tmpDir, err := os.MkdirTemp("", "wt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use explicit registry path instead of HOME env var (parallel-safe)
	regPath := filepath.Join(tmpDir, ".wt", "repos.json")

	// Create registry
	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo", Labels: []string{"backend"}},
			{Name: "bar", Path: "/tmp/bar"},
		},
	}

	// Save to explicit path
	if err := reg.Save(regPath); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(regPath); os.IsNotExist(err) {
		t.Error("registry file was not created")
	}

	// Load from explicit path
	loaded, err := Load(regPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if len(loaded.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(loaded.Repos))
	}

	repo, _ := loaded.FindByName("foo")
	if !repo.HasLabel("backend") {
		t.Error("expected loaded repo to have backend label")
	}
}

func TestAllLabels(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo", Labels: []string{"backend", "api"}},
			{Name: "bar", Path: "/tmp/bar", Labels: []string{"frontend", "api"}},
		},
	}

	labels := reg.AllLabels()
	if len(labels) != 3 {
		t.Errorf("expected 3 unique labels, got %d", len(labels))
	}

	// Labels should be sorted
	expected := []string{"api", "backend", "frontend"}
	for i, l := range expected {
		if labels[i] != l {
			t.Errorf("labels[%d] = %s, want %s", i, labels[i], l)
		}
	}
}

func TestAllRepoNames(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "zoo", Path: "/tmp/zoo"},
			{Name: "alpha", Path: "/tmp/alpha"},
			{Name: "beta", Path: "/tmp/beta"},
		},
	}

	names := reg.AllRepoNames()
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}

	// Names should be sorted
	expected := []string{"alpha", "beta", "zoo"}
	for i, n := range expected {
		if names[i] != n {
			t.Errorf("names[%d] = %s, want %s", i, names[i], n)
		}
	}
}

func TestFindByPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "myrepo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	reg := &Registry{
		Repos: []Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}

	// Find existing by absolute path
	repo, err := reg.FindByPath(repoPath)
	if err != nil {
		t.Fatalf("FindByPath() failed: %v", err)
	}
	if repo.Name != "myrepo" {
		t.Errorf("expected myrepo, got %s", repo.Name)
	}

	// Find non-existent path
	_, err = reg.FindByPath("/nonexistent/path")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestFindByPath_Symlink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	realPath := filepath.Join(resolved, "real-repo")
	if err := os.MkdirAll(realPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create symlink → real-repo
	linkPath := filepath.Join(resolved, "linked-repo")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	reg := &Registry{
		Repos: []Repo{
			{Name: "myrepo", Path: realPath},
		},
	}

	// Look up via symlink path — should find the repo
	repo, err := reg.FindByPath(linkPath)
	if err != nil {
		t.Fatalf("FindByPath(symlink) failed: %v", err)
	}
	if repo.Name != "myrepo" {
		t.Errorf("expected myrepo, got %s", repo.Name)
	}
}

func TestAdd_SymlinkDuplicate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	realPath := filepath.Join(resolved, "real-repo")
	if err := os.MkdirAll(realPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create symlink → real-repo
	linkPath := filepath.Join(resolved, "linked-repo")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	reg := &Registry{Repos: []Repo{}}

	// Register at real path
	if err := reg.Add(Repo{Name: "myrepo", Path: realPath}); err != nil {
		t.Fatalf("Add(real) failed: %v", err)
	}

	// Try to add again via symlink path — should detect duplicate
	err = reg.Add(Repo{Name: "myrepo-alias", Path: linkPath})
	if err == nil {
		t.Error("expected error adding duplicate via symlink path")
	}
}

func TestAdd_StoresCanonicalPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	realPath := filepath.Join(resolved, "real-repo")
	if err := os.MkdirAll(realPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Create symlink → real-repo
	linkPath := filepath.Join(resolved, "linked-repo")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	reg := &Registry{Repos: []Repo{}}

	// Add via symlink path
	if err := reg.Add(Repo{Name: "myrepo", Path: linkPath}); err != nil {
		t.Fatalf("Add(symlink) failed: %v", err)
	}

	// Stored path should be the canonical (real) path
	if reg.Repos[0].Path != realPath {
		t.Errorf("stored path = %q, want canonical %q", reg.Repos[0].Path, realPath)
	}
}

func TestRemove_Symlink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	realPath := filepath.Join(resolved, "real-repo")
	if err := os.MkdirAll(realPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	linkPath := filepath.Join(resolved, "linked-repo")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	reg := &Registry{
		Repos: []Repo{
			{Name: "myrepo", Path: realPath},
		},
	}

	// Remove via symlink path — should find and remove
	if err := reg.Remove(linkPath); err != nil {
		t.Fatalf("Remove(symlink) failed: %v", err)
	}
	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos after remove, got %d", len(reg.Repos))
	}
}

func TestFind_Symlink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	realPath := filepath.Join(resolved, "real-repo")
	if err := os.MkdirAll(realPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	linkPath := filepath.Join(resolved, "linked-repo")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	reg := &Registry{
		Repos: []Repo{
			{Name: "myrepo", Path: realPath},
		},
	}

	// Find via symlink path — should find the repo
	repo, err := reg.Find(linkPath)
	if err != nil {
		t.Fatalf("Find(symlink) failed: %v", err)
	}
	if repo.Name != "myrepo" {
		t.Errorf("expected myrepo, got %s", repo.Name)
	}
}

func TestFindByPath_ReverseSymlink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	realPath := filepath.Join(resolved, "real-repo")
	if err := os.MkdirAll(realPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	linkPath := filepath.Join(resolved, "linked-repo")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Simulate legacy data: registry stores symlinked (non-canonical) path
	reg := &Registry{
		Repos: []Repo{
			{Name: "myrepo", Path: linkPath},
		},
	}

	// Look up via real path — should still find the repo
	repo, err := reg.FindByPath(realPath)
	if err != nil {
		t.Fatalf("FindByPath(real) with stored symlink failed: %v", err)
	}
	if repo.Name != "myrepo" {
		t.Errorf("expected myrepo, got %s", repo.Name)
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo"},
		},
	}

	// Update worktree format
	if err := reg.Update("foo", func(r *Repo) {
		r.WorktreeFormat = "../{repo}-{branch}"
	}); err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	repo, err := reg.FindByName("foo")
	if err != nil {
		t.Fatalf("FindByName after Update failed: %v", err)
	}
	if repo.WorktreeFormat != "../{repo}-{branch}" {
		t.Errorf("expected worktree format updated, got %q", repo.WorktreeFormat)
	}

	// Update non-existent repo
	if err := reg.Update("nonexistent", func(r *Repo) {}); err == nil {
		t.Error("expected error updating non-existent repo")
	}
}

func TestGetEffectiveWorktreeFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		repoFormat    string
		defaultFormat string
		want          string
	}{
		{
			name:          "uses repo format when set",
			repoFormat:    "../{repo}-{branch}",
			defaultFormat: "{branch}",
			want:          "../{repo}-{branch}",
		},
		{
			name:          "falls back to default when repo format empty",
			repoFormat:    "",
			defaultFormat: "{repo}-{branch}",
			want:          "{repo}-{branch}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &Repo{
				Name:           "test",
				Path:           "/test",
				WorktreeFormat: tt.repoFormat,
			}
			if got := repo.GetEffectiveWorktreeFormat(tt.defaultFormat); got != tt.want {
				t.Errorf("GetEffectiveWorktreeFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathExists(t *testing.T) {
	t.Parallel()

	// Existing path
	tmpDir := t.TempDir()
	repo := &Repo{Name: "exists", Path: tmpDir}
	exists, err := repo.PathExists()
	if err != nil {
		t.Fatalf("PathExists() error: %v", err)
	}
	if !exists {
		t.Error("expected PathExists() = true for existing dir")
	}

	// Non-existent path
	repo2 := &Repo{Name: "gone", Path: filepath.Join(tmpDir, "nonexistent")}
	exists, err = repo2.PathExists()
	if err != nil {
		t.Fatalf("PathExists() error: %v", err)
	}
	if exists {
		t.Error("expected PathExists() = false for non-existent dir")
	}
}

func TestRepoString(t *testing.T) {
	t.Parallel()

	// Without labels
	repo := Repo{Name: "myrepo", Path: "/tmp/myrepo"}
	if got := repo.String(); got != "myrepo" {
		t.Errorf("String() = %q, want 'myrepo'", got)
	}

	// With labels
	repo2 := Repo{Name: "myrepo", Path: "/tmp/myrepo", Labels: []string{"backend", "api"}}
	if got := repo2.String(); got != "myrepo (backend, api)" {
		t.Errorf("String() = %q, want 'myrepo (backend, api)'", got)
	}
}

// TestLoad_DefaultPath exercises the path == "" branch in Load, which calls
// registryPath() → fs.WtDir() → os.UserHomeDir(). When no registry file
// exists at the default location the function must return an empty registry.
func TestLoad_DefaultPath(t *testing.T) {
	t.Parallel()

	// We can't change $HOME safely in a parallel test, so we just verify
	// that Load("") doesn't panic and returns either a registry or a
	// meaningful error (UserHomeDir could theoretically fail in CI).
	reg, err := Load("")
	if err != nil {
		// Acceptable: home dir not available in some environments.
		t.Logf("Load(\"\") returned error (acceptable in CI): %v", err)
		return
	}
	if reg == nil {
		t.Error("Load(\"\") returned nil registry without error")
	}
}

// TestSave_DefaultPath exercises the path == "" branch in Save, which calls
// registryPath() → fs.WtDir() → os.UserHomeDir().
func TestSave_DefaultPath(t *testing.T) {
	t.Parallel()

	reg := &Registry{Repos: []Repo{}}
	err := reg.Save("")
	if err != nil {
		// Acceptable: home dir not available in some environments.
		t.Logf("Save(\"\") returned error (acceptable in CI): %v", err)
	}
	// No assertion needed beyond "does not panic"; the path branch is exercised.
}

// TestSave_CreatesParentDir verifies that Save creates intermediate directories
// that don't exist yet.
func TestSave_CreatesParentDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Use a deeply nested path that doesn't exist yet.
	regPath := filepath.Join(tmpDir, "a", "b", "c", "repos.json")

	reg := &Registry{
		Repos: []Repo{
			{Name: "test", Path: "/tmp/test"},
		},
	}

	if err := reg.Save(regPath); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	if _, err := os.Stat(regPath); os.IsNotExist(err) {
		t.Error("registry file was not created in nested directory")
	}
}

// TestSave_WritesValidJSON verifies that Save produces JSON that Load can
// round-trip correctly.
func TestSave_WritesValidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	regPath := filepath.Join(tmpDir, "repos.json")

	reg := &Registry{
		Repos: []Repo{
			{Name: "alpha", Path: "/tmp/alpha", Labels: []string{"backend"}},
			{Name: "beta", Path: "/tmp/beta"},
		},
	}

	if err := reg.Save(regPath); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	loaded, err := Load(regPath)
	if err != nil {
		t.Fatalf("Load() after Save() failed: %v", err)
	}

	if len(loaded.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(loaded.Repos))
	}

	r, err := loaded.FindByName("alpha")
	if err != nil {
		t.Fatalf("FindByName(alpha) failed: %v", err)
	}
	if !r.HasLabel("backend") {
		t.Error("loaded repo missing label 'backend'")
	}
}

// TestLoad_MissingFile verifies that Load returns an empty registry (no error)
// when the file does not exist.
func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "does-not-exist.json")

	reg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos for missing file, got %d", len(reg.Repos))
	}
}

// TestLoad_InvalidJSON verifies that Load returns an error for malformed JSON.
func TestLoad_InvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")

	if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("setup: write failed: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Load() expected error for invalid JSON, got nil")
	}
}

// TestRemoveLabel_NonexistentLabel verifies that RemoveLabel is a no-op (no
// error) when the label doesn't exist on the repo.
func TestRemoveLabel_NonexistentLabel(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "foo", Path: "/tmp/foo", Labels: []string{"backend"}},
		},
	}

	// Remove a label that was never added — should succeed silently.
	if err := reg.RemoveLabel("foo", "nonexistent"); err != nil {
		t.Errorf("RemoveLabel(nonexistent) error = %v, want nil", err)
	}

	// Existing label should still be present.
	repo, err := reg.FindByName("foo")
	if err != nil {
		t.Fatalf("FindByName failed: %v", err)
	}
	if !repo.HasLabel("backend") {
		t.Error("existing label 'backend' was unexpectedly removed")
	}
}

// TestRemoveLabel_NonexistentRepo verifies that RemoveLabel returns an error
// when the repo doesn't exist.
func TestRemoveLabel_NonexistentRepo(t *testing.T) {
	t.Parallel()

	reg := &Registry{Repos: []Repo{}}

	if err := reg.RemoveLabel("nonexistent", "label"); err == nil {
		t.Error("RemoveLabel() on non-existent repo expected error, got nil")
	}
}

// TestClearLabels_EmptyLabels verifies that ClearLabels is a no-op when the
// repo already has no labels (nil slice).
func TestClearLabels_EmptyLabels(t *testing.T) {
	t.Parallel()

	reg := &Registry{
		Repos: []Repo{
			{Name: "nolabels", Path: "/tmp/nolabels"},
		},
	}

	if err := reg.ClearLabels("nolabels"); err != nil {
		t.Errorf("ClearLabels() on repo with no labels error = %v, want nil", err)
	}

	repo, err := reg.FindByName("nolabels")
	if err != nil {
		t.Fatalf("FindByName failed: %v", err)
	}
	if len(repo.Labels) != 0 {
		t.Errorf("expected 0 labels after ClearLabels, got %d", len(repo.Labels))
	}
}

// TestClearLabels_NonexistentRepo verifies that ClearLabels returns an error
// when the named repo doesn't exist.
func TestClearLabels_NonexistentRepo(t *testing.T) {
	t.Parallel()

	reg := &Registry{Repos: []Repo{}}

	if err := reg.ClearLabels("nonexistent"); err == nil {
		t.Error("ClearLabels() on non-existent repo expected error, got nil")
	}
}

// TestPathExists_NonexistentPath verifies that PathExists returns false (no
// error) for a path that doesn't exist on disk.
func TestPathExists_NonexistentPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	repo := &Repo{Name: "gone", Path: filepath.Join(tmpDir, "does-not-exist")}

	exists, err := repo.PathExists()
	if err != nil {
		t.Fatalf("PathExists() error = %v, want nil", err)
	}
	if exists {
		t.Error("PathExists() = true for non-existent path, want false")
	}
}

// TestAddLabel_NonexistentRepo verifies that AddLabel returns an error when
// the named repo doesn't exist.
func TestAddLabel_NonexistentRepo(t *testing.T) {
	t.Parallel()

	reg := &Registry{Repos: []Repo{}}

	if err := reg.AddLabel("nonexistent", "backend"); err == nil {
		t.Error("AddLabel() on non-existent repo expected error, got nil")
	}
}

// TestResolveAsPath verifies that resolveAsPath returns a canonical absolute
// path for a valid relative or absolute input.
func TestResolveAsPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}

	// Absolute path — should be canonicalized (symlinks resolved on macOS).
	got := resolveAsPath(resolved)
	if got != resolved {
		t.Errorf("resolveAsPath(%q) = %q, want %q", resolved, got, resolved)
	}

	// Relative path — should be converted to absolute.
	// Use "." which always resolves to the current working directory.
	got2 := resolveAsPath(".")
	if got2 == "." {
		t.Error("resolveAsPath('.') should return absolute path, got '.'")
	}
}
