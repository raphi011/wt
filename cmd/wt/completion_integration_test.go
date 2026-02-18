//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// completionTestRoot creates a minimal root command with the completion subcommand.
// This is needed because `newCompletionCmd` calls `cmd.Root().GenXxxCompletion()`
// which requires a proper command tree.
func completionTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "wt",
		Short: "test root",
	}
	// Add the command group that completion_cmd.go requires (GroupConfig)
	root.AddGroup(&cobra.Group{ID: GroupConfig, Title: "Configuration"})
	root.AddCommand(newCompletionCmd())
	return root
}

// TestCompletion_Fish tests that fish completion generation succeeds.
//
// Scenario: User runs `wt completion fish`
// Expected: Command succeeds without error
func TestCompletion_Fish(t *testing.T) {
	t.Parallel()

	root := completionTestRoot()
	root.SetArgs([]string{"completion", "fish"})

	// completion outputs via os.Stdout directly, so we verify no error
	if err := root.Execute(); err != nil {
		t.Fatalf("completion fish failed: %v", err)
	}
}

// TestCompletion_Bash tests that bash completion generation succeeds.
//
// Scenario: User runs `wt completion bash`
// Expected: Command succeeds without error
func TestCompletion_Bash(t *testing.T) {
	t.Parallel()

	root := completionTestRoot()
	root.SetArgs([]string{"completion", "bash"})

	if err := root.Execute(); err != nil {
		t.Fatalf("completion bash failed: %v", err)
	}
}

// TestCompletion_Zsh tests that zsh completion generation succeeds.
//
// Scenario: User runs `wt completion zsh`
// Expected: Command succeeds without error
func TestCompletion_Zsh(t *testing.T) {
	t.Parallel()

	root := completionTestRoot()
	root.SetArgs([]string{"completion", "zsh"})

	if err := root.Execute(); err != nil {
		t.Fatalf("completion zsh failed: %v", err)
	}
}

// TestCompleteBranches_WithContext tests that completeBranches uses cmd.Context()
// to resolve the working directory and return matching branch names.
//
// Scenario: completeBranches is called with a context containing config and workDir
// Expected: Returns local branch names from the repo at workDir
func TestCompleteBranches_WithContext(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature-a", "feature-b"})

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(ctx)

	matches, directive := completeBranches(cmd, nil, "feature")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %d", directive)
	}

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches for 'feature' prefix, got %d: %v", len(matches), matches)
	}

	found := map[string]bool{}
	for _, m := range matches {
		found[m] = true
	}
	if !found["feature-a"] || !found["feature-b"] {
		t.Errorf("expected feature-a and feature-b in matches, got %v", matches)
	}
}

// TestCompleteBaseBranches_WithContext tests that completeBaseBranches uses cmd.Context()
// to resolve the working directory and return matching branch names and remote prefixes.
//
// Scenario: completeBaseBranches is called with a context containing config and workDir
// Expected: Returns local branch names and remote prefixes from the repo at workDir
func TestCompleteBaseBranches_WithContext(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"develop"})

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(ctx)

	matches, directive := completeBaseBranches(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %d", directive)
	}

	// Should include at least "main" and "develop" as local branches,
	// plus "origin/" as a remote prefix
	found := map[string]bool{}
	for _, m := range matches {
		found[m] = true
	}
	if !found["main"] {
		t.Errorf("expected 'main' in matches, got %v", matches)
	}
	if !found["develop"] {
		t.Errorf("expected 'develop' in matches, got %v", matches)
	}
	if !found["origin/"] {
		t.Errorf("expected 'origin/' remote prefix in matches, got %v", matches)
	}
}
