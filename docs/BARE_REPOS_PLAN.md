# Bare Git Repos Migration Plan

This document outlines the migration from the current dual-directory architecture (`repo_dir` + `worktree_dir`) to a bare git repository model where all worktrees live underneath their parent bare repo.

## Current vs New Architecture

### Current Architecture
```
~/Git/                          # repo_dir (repos with working dirs)
├── project-a/                  # Main repo (has .git directory)
│   └── .git/
├── project-b/
│   └── .git/

~/Git/worktrees/                # worktree_dir (flat list)
├── project-a-main/             # Worktree (.git file)
├── project-a-feature-x/
├── project-a-bugfix-y/
├── project-b-main/
└── project-b-feature-z/
```

**Problems:**
- Two directories to configure and manage
- Worktrees scattered in flat structure
- No clear visual grouping by project
- Main repo has a working directory that's rarely used

### New Architecture (Bare Repos)
```
~/Git/                          # Single base_dir
├── project-a.git/              # Bare repo (no working dir)
│   ├── HEAD
│   ├── config
│   ├── objects/
│   ├── refs/
│   ├── worktrees/              # Git's internal worktree metadata
│   └── trees/                  # Our worktree directories
│       ├── main/               # Worktree for main branch
│       ├── feature-x/          # Worktree for feature-x
│       └── bugfix-y/
├── project-b.git/
│   └── trees/
│       ├── main/
│       └── feature-z/
```

**Benefits:**
- Single directory to configure
- Clear project grouping
- No unused working directory in main repo
- Matches how many developers use worktrees
- Cleaner mental model

---

## Implementation Phases

### Phase 1: Core Infrastructure Changes

#### 1.1 Config Changes

**File:** `internal/config/config.go`

```go
// Old config
type Config struct {
    WorktreeDir    string  // DEPRECATED
    RepoDir        string  // DEPRECATED
    // ...
}

// New config
type Config struct {
    BaseDir        string  // Single directory for all bare repos
    WorktreeSubdir string  // Subdirectory name for worktrees (default: "trees")

    // Deprecated (for migration)
    WorktreeDir    string  `toml:"worktree_dir,omitempty"`
    RepoDir        string  `toml:"repo_dir,omitempty"`
    // ...
}
```

**New config file format:**
```toml
# ~/.config/wt/config.toml
base_dir = "~/Git"              # All bare repos live here
worktree_subdir = "trees"       # Worktrees in {repo}.git/trees/

# Legacy (still supported for migration period)
# worktree_dir = "~/Git/worktrees"
# repo_dir = "~/Git"
```

**Environment variables:**
- `WT_BASE_DIR` - New primary env var
- `WT_WORKTREE_DIR` / `WT_REPO_DIR` - Deprecated, trigger migration warning

**Tasks:**
- [ ] Add `BaseDir` and `WorktreeSubdir` to Config struct
- [ ] Add `WT_BASE_DIR` env var support
- [ ] Add migration detection (if old vars set, warn user)
- [ ] Update `config.ValidatePath()` for new fields
- [ ] Add helper `Config.BareRepoPath(repoName)` → `{base_dir}/{repo}.git`
- [ ] Add helper `Config.WorktreePath(repoName, branch)` → `{base_dir}/{repo}.git/trees/{branch}`

---

#### 1.2 Bare Repo Detection

**File:** `internal/git/repo.go`

Add functions to detect and work with bare repos:

```go
// IsBareRepo checks if a directory is a bare git repository
func IsBareRepo(path string) (bool, error)

// FindAllBareRepos scans a directory for bare repos (*.git directories)
func FindAllBareRepos(baseDir string) ([]string, error)

// GetBareRepoName extracts repo name from bare repo path
// "project-a.git" → "project-a"
func GetBareRepoName(bareRepoPath string) string
```

**Detection logic:**
```go
func IsBareRepo(path string) (bool, error) {
    // Check if directory ends with .git
    if !strings.HasSuffix(path, ".git") {
        return false, nil
    }

    // Check for bare repo markers: HEAD file at root, no .git subdirectory
    headPath := filepath.Join(path, "HEAD")
    if _, err := os.Stat(headPath); err != nil {
        return false, nil
    }

    // Verify it's actually bare (no worktree)
    cmd := exec.Command("git", "-C", path, "rev-parse", "--is-bare-repository")
    out, err := cmd.Output()
    return strings.TrimSpace(string(out)) == "true", err
}
```

**Tasks:**
- [ ] Implement `IsBareRepo()`
- [ ] Implement `FindAllBareRepos()`
- [ ] Implement `GetBareRepoName()`
- [ ] Add unit tests for bare repo detection

---

#### 1.3 Worktree Discovery Updates

**File:** `internal/git/worktree.go`

Update worktree discovery to work with bare repos:

```go
// ListWorktreesForBareRepo lists all worktrees for a bare repo
func ListWorktreesForBareRepo(bareRepoPath string) ([]Worktree, error)

// FindWorktreeInBareRepo finds a specific worktree by branch name
func FindWorktreeInBareRepo(bareRepoPath, branch string) (*Worktree, error)
```

**Updated `git worktree list` parsing:**
- When run from bare repo, returns worktrees in `trees/` subdirectory
- Need to handle path resolution correctly

**Tasks:**
- [ ] Update `ListWorktrees()` to support bare repos
- [ ] Add `ListWorktreesForBareRepo()` helper
- [ ] Update `Worktree` struct if needed (add `BareRepoPath`?)
- [ ] Update worktree path resolution logic

---

### Phase 2: Clone Command

#### 2.1 New Clone Command

**File:** `cmd/wt/clone.go` (new file)

```go
type CloneCmd struct {
    Deps

    RepoURL     string `arg:"" help:"Repository URL or owner/repo shorthand"`
    Name        string `short:"n" help:"Override repo name (default: derived from URL)"`
    NoWorktree  bool   `short:"N" help:"Clone bare repo only, don't create initial worktree"`
}
```

**Behavior:**
```bash
# Clone as bare repo + create main worktree
wt clone github.com/user/project
# Result:
# ~/Git/project.git/           (bare repo)
# ~/Git/project.git/trees/main/ (worktree on default branch)

# Clone with custom name
wt clone github.com/user/project -n my-project
# Result: ~/Git/my-project.git/

# Clone bare only (no worktree)
wt clone github.com/user/project -N
# Result: ~/Git/project.git/ (no trees/ yet)
```

**Implementation:**
```go
func (c *CloneCmd) Run(ctx context.Context) error {
    cfg := c.Config
    repoName := c.Name
    if repoName == "" {
        repoName = extractRepoName(c.RepoURL)
    }

    bareRepoPath := filepath.Join(cfg.BaseDir, repoName+".git")

    // Clone as bare
    // git clone --bare <url> <path>
    if err := git.CloneBare(ctx, c.RepoURL, bareRepoPath); err != nil {
        return err
    }

    // Configure remote for fetch (bare repos need this)
    // git config remote.origin.fetch "+refs/heads/*:refs/remotes/origin/*"
    if err := git.ConfigureBareRemote(ctx, bareRepoPath); err != nil {
        return err
    }

    // Create initial worktree unless --no-worktree
    if !c.NoWorktree {
        defaultBranch := git.GetDefaultBranch(ctx, bareRepoPath)
        treesDir := filepath.Join(bareRepoPath, cfg.WorktreeSubdir)
        worktreePath := filepath.Join(treesDir, defaultBranch)

        if err := git.AddWorktree(ctx, bareRepoPath, worktreePath, defaultBranch); err != nil {
            return err
        }
    }

    return nil
}
```

**Tasks:**
- [ ] Create `cmd/wt/clone.go`
- [ ] Add `git.CloneBare()` function
- [ ] Add `git.ConfigureBareRemote()` function
- [ ] Add to kong CLI parser in `main.go`
- [ ] Update shell completions
- [ ] Add integration tests

---

#### 2.2 Git Clone Helpers

**File:** `internal/git/clone.go` (new file)

```go
// CloneBare clones a repository as a bare repo
func CloneBare(ctx context.Context, url, destPath string) error {
    cmd := exec.CommandContext(ctx, "git", "clone", "--bare", url, destPath)
    return cmd.Run()
}

// ConfigureBareRemote sets up fetch refspec for a bare repo
// By default, bare repos don't fetch remote branches properly
func ConfigureBareRemote(ctx context.Context, bareRepoPath string) error {
    cmd := exec.CommandContext(ctx, "git", "-C", bareRepoPath,
        "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
    return cmd.Run()
}
```

**Tasks:**
- [ ] Create `internal/git/clone.go`
- [ ] Implement `CloneBare()`
- [ ] Implement `ConfigureBareRemote()`
- [ ] Add unit tests

---

### Phase 3: Checkout Command Updates

#### 3.1 Update Worktree Creation

**File:** `cmd/wt/checkout.go`

Update `createWorktree()` to use bare repo structure:

```go
func (c *CheckoutCmd) createWorktree(ctx context.Context, repo *git.Repo, branch string) error {
    cfg := c.Config

    // Determine paths based on repo type
    var worktreePath string
    if repo.IsBare {
        // New style: worktree inside bare repo
        treesDir := filepath.Join(repo.Path, cfg.WorktreeSubdir)
        worktreePath = filepath.Join(treesDir, sanitizeBranchName(branch))
    } else {
        // Legacy style: worktree in worktree_dir
        worktreePath = filepath.Join(cfg.WorktreeDir, fmt.Sprintf("%s-%s", repo.Name, branch))
    }

    // ... rest of creation logic
}
```

**Branch name sanitization:**
```go
// sanitizeBranchName converts branch name to valid directory name
// "feature/add-login" → "feature-add-login"
// "user/name/branch" → "user-name-branch"
func sanitizeBranchName(branch string) string {
    return strings.ReplaceAll(branch, "/", "-")
}
```

**Tasks:**
- [ ] Update `createWorktree()` for bare repo paths
- [ ] Add `sanitizeBranchName()` helper
- [ ] Update path detection logic
- [ ] Handle mixed mode (some bare, some regular repos)
- [ ] Update integration tests

---

#### 3.2 Update PR Checkout

**File:** `cmd/wt/pr.go`

Update `PrCheckoutCmd` to:
1. Clone as bare if repo doesn't exist
2. Create worktree in bare repo's trees/ directory

```go
func (c *PrCheckoutCmd) runPrCheckout(ctx context.Context) error {
    // ... existing PR info fetching ...

    repoName := extractRepoName(c.Repo)
    bareRepoPath := filepath.Join(cfg.BaseDir, repoName+".git")

    // Clone if not exists
    if !exists(bareRepoPath) {
        if err := git.CloneBare(ctx, repoURL, bareRepoPath); err != nil {
            return err
        }
        git.ConfigureBareRemote(ctx, bareRepoPath)
    }

    // Fetch PR branch
    if err := git.FetchPRBranch(ctx, bareRepoPath, prInfo); err != nil {
        return err
    }

    // Create worktree
    treesDir := filepath.Join(bareRepoPath, cfg.WorktreeSubdir)
    worktreePath := filepath.Join(treesDir, sanitizeBranchName(prInfo.Branch))

    return git.AddWorktree(ctx, bareRepoPath, worktreePath, prInfo.Branch)
}
```

**Tasks:**
- [ ] Update `PrCheckoutCmd.runPrCheckout()`
- [ ] Add `git.FetchPRBranch()` for bare repos
- [ ] Update forge `CloneRepo()` to support bare cloning
- [ ] Update integration tests

---

### Phase 4: List Command Updates

#### 4.1 Update Discovery

**File:** `cmd/wt/list.go`

Update `ListCmd` to discover bare repos and their worktrees:

```go
func (c *ListCmd) Run(ctx context.Context) error {
    cfg := c.Config

    var allWorktrees []git.Worktree

    // Find all bare repos
    bareRepos, err := git.FindAllBareRepos(cfg.BaseDir)
    if err != nil {
        return err
    }

    for _, bareRepo := range bareRepos {
        worktrees, err := git.ListWorktreesForBareRepo(bareRepo)
        if err != nil {
            continue
        }
        allWorktrees = append(allWorktrees, worktrees...)
    }

    // Also check legacy locations if configured
    if cfg.WorktreeDir != "" {
        legacyWorktrees, _ := git.ListWorktrees(cfg.WorktreeDir)
        allWorktrees = append(allWorktrees, legacyWorktrees...)
    }

    // ... render table ...
}
```

**Tasks:**
- [ ] Update `ListCmd.Run()` for bare repo discovery
- [ ] Support mixed mode (bare + legacy)
- [ ] Update table rendering if needed
- [ ] Update integration tests

---

### Phase 5: Other Command Updates

#### 5.1 Commands Requiring Updates

| Command | Changes Needed |
|---------|---------------|
| `wt cd` | Update path resolution for bare repos |
| `wt exec` | Update worktree discovery |
| `wt hook` | Update worktree path resolution |
| `wt prune` | Update to prune from bare repo structure |
| `wt mv` | Major rewrite (see below) |
| `wt note` | Update worktree lookup |
| `wt pr create` | Update to work from worktree in bare repo |
| `wt pr merge` | Update worktree lookup |

#### 5.2 Move Command Rewrite

**File:** `cmd/wt/mv.go`

The `mv` command needs significant changes:

**New behavior:**
- Move a worktree between bare repos (unlikely use case)
- Move a worktree within a bare repo (rename)
- **Migration mode**: Convert regular repo to bare repo structure

```go
type MvCmd struct {
    Deps

    // New flags
    ToBare    bool `long:"to-bare" help:"Convert regular repo to bare repo structure"`
    Cascade   bool `long:"cascade" help:"Move repo and all its worktrees"`
}
```

**Tasks:**
- [ ] Rewrite `MvCmd` for bare repo support
- [ ] Add `--to-bare` migration flag
- [ ] Update `--cascade` for bare repo structure
- [ ] Add integration tests

---

### Phase 6: Migration Tool

#### 6.1 Migration Command

**File:** `cmd/wt/migrate.go` (new file)

```go
type MigrateCmd struct {
    Deps

    DryRun  bool `short:"d" help:"Show what would be migrated without making changes"`
    Force   bool `short:"f" help:"Force migration even if some worktrees have uncommitted changes"`
}
```

**Migration steps:**
1. Scan `repo_dir` for regular repos
2. For each repo:
   a. Create bare clone: `git clone --bare <repo> <repo>.git`
   b. Configure remote fetch refspec
   c. Find all worktrees for this repo in `worktree_dir`
   d. Move worktrees to `<repo>.git/trees/`
   e. Run `git worktree repair` to fix paths
   f. Remove original repo (after confirmation)

**Migration script logic:**
```go
func (c *MigrateCmd) Run(ctx context.Context) error {
    cfg := c.Config
    l := log.FromContext(ctx)

    // Find all regular repos
    repos, err := git.FindAllRepos(cfg.RepoDir)
    if err != nil {
        return err
    }

    for _, repo := range repos {
        repoName := filepath.Base(repo.Path)
        bareRepoPath := filepath.Join(cfg.BaseDir, repoName+".git")

        l.Info("Migrating", "repo", repoName)

        if c.DryRun {
            l.Info("Would create bare repo", "path", bareRepoPath)
            continue
        }

        // Step 1: Create bare repo from existing
        if err := c.convertToBare(ctx, repo.Path, bareRepoPath); err != nil {
            return fmt.Errorf("failed to convert %s: %w", repoName, err)
        }

        // Step 2: Move worktrees
        worktrees, _ := git.ListWorktreesForRepo(repo.Path)
        for _, wt := range worktrees {
            newPath := filepath.Join(bareRepoPath, cfg.WorktreeSubdir,
                sanitizeBranchName(wt.Branch))

            if err := c.moveWorktree(ctx, wt.Path, newPath, bareRepoPath); err != nil {
                l.Warn("Failed to move worktree", "path", wt.Path, "error", err)
            }
        }

        // Step 3: Repair worktree links
        git.RepairWorktrees(ctx, bareRepoPath)

        // Step 4: Remove old repo (with confirmation)
        if !c.Force {
            if !ui.Confirm("Remove original repo at %s?", repo.Path) {
                continue
            }
        }
        os.RemoveAll(repo.Path)
    }

    // Update config file
    return c.updateConfig(ctx)
}

func (c *MigrateCmd) convertToBare(ctx context.Context, repoPath, bareRepoPath string) error {
    // Option 1: Clone as bare from local repo
    // git clone --bare <repo> <bare>

    // Option 2: Convert in-place (more complex)
    // - Move .git to new location
    // - Convert to bare
    // - Update config

    // Using option 1 for simplicity
    cmd := exec.CommandContext(ctx, "git", "clone", "--bare", repoPath, bareRepoPath)
    return cmd.Run()
}

func (c *MigrateCmd) moveWorktree(ctx context.Context, oldPath, newPath, bareRepoPath string) error {
    // Create trees directory if needed
    treesDir := filepath.Dir(newPath)
    os.MkdirAll(treesDir, 0755)

    // Move the worktree directory
    if err := os.Rename(oldPath, newPath); err != nil {
        return err
    }

    // Update the .git file in the worktree to point to new bare repo
    gitFile := filepath.Join(newPath, ".git")
    worktreeName := filepath.Base(newPath)
    newGitdir := filepath.Join(bareRepoPath, "worktrees", worktreeName)

    content := fmt.Sprintf("gitdir: %s\n", newGitdir)
    return os.WriteFile(gitFile, []byte(content), 0644)
}
```

**Tasks:**
- [ ] Create `cmd/wt/migrate.go`
- [ ] Implement `convertToBare()`
- [ ] Implement `moveWorktree()`
- [ ] Add `git.RepairWorktrees()` helper
- [ ] Add dry-run mode
- [ ] Add progress output
- [ ] Handle edge cases (dirty worktrees, conflicts)
- [ ] Add integration tests
- [ ] Update shell completions

---

### Phase 7: Documentation & Deprecation

#### 7.1 Documentation Updates

- [ ] Update README.md with new architecture
- [ ] Add migration guide section
- [ ] Update example config
- [ ] Document new `wt clone` command
- [ ] Document `wt migrate` command

#### 7.2 Deprecation Warnings

Add warnings when using old config:

```go
func (c *Config) ValidateAndWarn(ctx context.Context) {
    l := log.FromContext(ctx)

    if c.WorktreeDir != "" || c.RepoDir != "" {
        l.Warn("worktree_dir and repo_dir are deprecated")
        l.Warn("Run 'wt migrate' to convert to bare repo structure")
        l.Warn("See: https://github.com/user/wt/docs/MIGRATION.md")
    }
}
```

**Tasks:**
- [ ] Add deprecation warnings
- [ ] Create migration documentation
- [ ] Update README.md
- [ ] Update help text for affected commands

---

## Implementation Order

### Sprint 1: Foundation
1. Config changes (1.1)
2. Bare repo detection (1.2)
3. Worktree discovery updates (1.3)

### Sprint 2: Core Commands
4. Clone command (2.1, 2.2)
5. Checkout updates (3.1)
6. PR checkout updates (3.2)

### Sprint 3: List & Discovery
7. List command updates (4.1)
8. Other command updates (5.1)

### Sprint 4: Migration
9. Move command rewrite (5.2)
10. Migration tool (6.1)

### Sprint 5: Polish
11. Documentation (7.1)
12. Deprecation warnings (7.2)
13. Final testing & edge cases

---

## Breaking Changes

| Change | Impact | Migration |
|--------|--------|-----------|
| New directory structure | All worktrees move | `wt migrate` command |
| Config format change | Config file update needed | Auto-detected, warns user |
| Worktree paths change | Hardcoded paths break | Use `wt cd` instead |
| Cache invalidation | IDs may change | Cache rebuilt automatically |

---

## Testing Strategy

### Unit Tests
- Bare repo detection
- Path sanitization
- Config parsing (old + new format)

### Integration Tests
- Clone bare repo
- Create worktree in bare repo
- List worktrees from bare repo
- Migration from regular to bare
- Mixed mode (some bare, some regular)

### Manual Testing Checklist
- [ ] Fresh install with new config
- [ ] Migration from existing setup
- [ ] GitHub workflow (clone, checkout PR, create PR, merge)
- [ ] GitLab workflow (same)
- [ ] Hooks work correctly
- [ ] Shell completions work
- [ ] `wt cd` works with new paths

---

## Open Questions

1. **Worktree naming**: Use branch name directly or include repo prefix?
   - `trees/feature-x` vs `trees/project-feature-x`
   - Recommendation: Just branch name (cleaner, repo context is parent dir)

2. **Default branch worktree**: Always create on clone, or optional?
   - Recommendation: Create by default, `--no-worktree` to skip

3. **Mixed mode support**: How long to support both structures?
   - Recommendation: Support indefinitely, but warn on old config

4. **Bare repo suffix**: `.git` required or optional?
   - Recommendation: Required (standard convention, clearer intent)

5. **Trees subdirectory name**: `trees/`, `worktrees/`, or configurable?
   - Recommendation: `trees/` default, configurable via `worktree_subdir`
