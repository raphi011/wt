//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestLabel_Add tests adding a label to a repo.
//
// Scenario: User runs `wt label add backend myrepo`
// Expected: Label is added to the repo
func TestLabel_Add(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"add", "backend", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label add command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, err := reg.FindByName("myrepo")
	if err != nil {
		t.Fatalf("failed to find repo: %v", err)
	}

	hasLabel := false
	for _, l := range repo.Labels {
		if l == "backend" {
			hasLabel = true
			break
		}
	}

	if !hasLabel {
		t.Errorf("expected repo to have label 'backend', got %v", repo.Labels)
	}
}

// TestLabel_Remove tests removing a label from a repo.
//
// Scenario: User runs `wt label remove backend myrepo`
// Expected: Label is removed from the repo
func TestLabel_Remove(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend", "api"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"remove", "backend", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label remove command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, err := reg.FindByName("myrepo")
	if err != nil {
		t.Fatalf("failed to find repo: %v", err)
	}

	for _, l := range repo.Labels {
		if l == "backend" {
			t.Errorf("expected label 'backend' to be removed, but found it in %v", repo.Labels)
		}
	}

	// Should still have api label
	hasAPI := false
	for _, l := range repo.Labels {
		if l == "api" {
			hasAPI = true
		}
	}
	if !hasAPI {
		t.Errorf("expected label 'api' to remain, got %v", repo.Labels)
	}
}

// TestLabel_List tests listing labels for a repo.
//
// Scenario: User runs `wt label list myrepo`
// Expected: Labels are listed
func TestLabel_List(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend", "api"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"list", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "backend") {
		t.Errorf("expected output to contain 'backend', got %q", output)
	}
	if !strings.Contains(output, "api") {
		t.Errorf("expected output to contain 'api', got %q", output)
	}
}

// TestLabel_Clear tests clearing all labels from a repo.
//
// Scenario: User runs `wt label clear myrepo`
// Expected: All labels are removed
func TestLabel_Clear(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend", "api"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"clear", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label clear command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, err := reg.FindByName("myrepo")
	if err != nil {
		t.Fatalf("failed to find repo: %v", err)
	}

	if len(repo.Labels) != 0 {
		t.Errorf("expected 0 labels after clear, got %d: %v", len(repo.Labels), repo.Labels)
	}
}

// TestLabel_Add_ByLabelScope tests adding a label using a label as scope.
//
// Scenario: User runs `wt label add newlabel backend` where "backend" is an existing label
// Expected: Label "newlabel" is added to all repos with label "backend"
func TestLabel_Add_ByLabelScope(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	repo1Path := setupTestRepo(t, tmpDir, "repo1")
	repo2Path := setupTestRepo(t, tmpDir, "repo2")
	setupTestRepo(t, tmpDir, "repo3")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path, Labels: []string{"backend"}},
			{Name: "repo2", Path: repo2Path, Labels: []string{"backend"}},
			{Name: "repo3", Path: filepath.Join(tmpDir, "repo3"), Labels: []string{"frontend"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"add", "newlabel", "backend"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label add command failed: %v", err)
	}

	// Reload and verify
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	for _, name := range []string{"repo1", "repo2"} {
		repo, err := reg.FindByName(name)
		if err != nil {
			t.Fatalf("failed to find repo %s: %v", name, err)
		}
		hasLabel := false
		for _, l := range repo.Labels {
			if l == "newlabel" {
				hasLabel = true
				break
			}
		}
		if !hasLabel {
			t.Errorf("expected %s to have label 'newlabel', got %v", name, repo.Labels)
		}
	}

	// repo3 should NOT have the new label
	repo3, _ := reg.FindByName("repo3")
	for _, l := range repo3.Labels {
		if l == "newlabel" {
			t.Errorf("repo3 should not have label 'newlabel', got %v", repo3.Labels)
		}
	}
}

// TestLabel_Add_DuplicateLabel tests that adding a label that already exists is idempotent.
//
// Scenario: User runs `wt label add backend myrepo` when myrepo already has "backend"
// Expected: No error, label is not duplicated
func TestLabel_Add_DuplicateLabel(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"add", "backend", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label add command failed: %v", err)
	}

	// Reload and verify label is not duplicated
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, _ := reg.FindByName("myrepo")
	count := 0
	for _, l := range repo.Labels {
		if l == "backend" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'backend' label, got %d in %v", count, repo.Labels)
	}
}

// TestLabel_Remove_LabelNotFound tests removing a label that doesn't exist on the repo.
//
// Scenario: User runs `wt label remove nonexistent myrepo`
// Expected: No error (idempotent removal)
func TestLabel_Remove_LabelNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"remove", "nonexistent", "myrepo"})

	// Should not error - removing a non-existent label is a no-op
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label remove command failed unexpectedly: %v", err)
	}

	// Verify original labels remain unchanged
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, _ := reg.FindByName("myrepo")
	if len(repo.Labels) != 1 || repo.Labels[0] != "backend" {
		t.Errorf("expected labels [backend], got %v", repo.Labels)
	}
}

// TestLabel_List_Global tests listing all labels across repos with --global flag.
//
// Scenario: User runs `wt label list -g` with multiple repos having different labels
// Expected: All unique labels are listed
func TestLabel_List_Global(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repo1Path := setupTestRepo(t, tmpDir, "repo1")
	repo2Path := setupTestRepo(t, tmpDir, "repo2")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path, Labels: []string{"backend", "api"}},
			{Name: "repo2", Path: repo2Path, Labels: []string{"frontend", "api"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, tmpDir)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"list", "-g"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label list --global command failed: %v", err)
	}

	output := out.String()
	for _, label := range []string{"backend", "frontend", "api"} {
		if !strings.Contains(output, label) {
			t.Errorf("expected output to contain label %q, got %q", label, output)
		}
	}
}

// TestLabel_List_NoLabels tests listing labels for a repo that has no labels.
//
// Scenario: User runs `wt label list myrepo` where myrepo has no labels
// Expected: Output shows "(no labels)"
func TestLabel_List_NoLabels(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, _ := testContextWithConfigAndOutput(t, cfg, tmpDir)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"list", "myrepo"})

	// Should succeed (prints "(no labels)" to stdout via fmt.Println)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("label list command failed: %v", err)
	}
}

// TestLabel_List_MultipleRepos tests listing labels for multiple repos at once.
//
// Scenario: User runs `wt label list repo1 repo2`
// Expected: Labels are printed with repo name prefixes
func TestLabel_List_MultipleRepos(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repo1Path := setupTestRepo(t, tmpDir, "repo1")
	repo2Path := setupTestRepo(t, tmpDir, "repo2")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path, Labels: []string{"backend"}},
			{Name: "repo2", Path: repo2Path, Labels: []string{"frontend"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, tmpDir)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"list", "repo1", "repo2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label list command failed: %v", err)
	}

	output := out.String()
	// When listing multiple repos, output.Println writes labels
	// fmt.Printf writes the prefix to stdout (not captured)
	// But the labels themselves should appear in captured output
	if !strings.Contains(output, "backend") {
		t.Errorf("expected output to contain 'backend', got %q", output)
	}
	if !strings.Contains(output, "frontend") {
		t.Errorf("expected output to contain 'frontend', got %q", output)
	}
}

// TestLabel_Add_CurrentRepo tests adding a label when no scope is provided.
//
// Scenario: User runs `wt label add mytag` while inside a registered repo
// Expected: Label is added to the current repo (auto-detected from workDir)
func TestLabel_Add_CurrentRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"add", "mytag"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label add command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, err := reg.FindByName("myrepo")
	if err != nil {
		t.Fatalf("failed to find repo: %v", err)
	}

	hasLabel := false
	for _, l := range repo.Labels {
		if l == "mytag" {
			hasLabel = true
			break
		}
	}

	if !hasLabel {
		t.Errorf("expected repo to have label 'mytag', got %v", repo.Labels)
	}
}

// TestLabel_Remove_NotInGitRepo tests error when removing a label from outside a git repo.
//
// Scenario: User runs `wt label remove backend` from a non-git directory
// Expected: Returns "not in a git repository" error
func TestLabel_Remove_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Use a non-git directory as workDir
	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, nonGitDir)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"remove", "backend"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestLabel_Clear_CurrentRepo tests clearing labels when no scope is provided.
//
// Scenario: User runs `wt label clear` while inside a registered repo with labels
// Expected: All labels are cleared from the current repo (auto-detected from workDir)
func TestLabel_Clear_CurrentRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend", "api"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"clear"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label clear command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, err := reg.FindByName("myrepo")
	if err != nil {
		t.Fatalf("failed to find repo: %v", err)
	}

	if len(repo.Labels) != 0 {
		t.Errorf("expected 0 labels after clear, got %d: %v", len(repo.Labels), repo.Labels)
	}
}
