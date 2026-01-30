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
│   ├── HEAD                    # Git internal files
│   ├── config
│   ├── objects/
│   ├── refs/
│   ├── worktrees/              # Git's internal worktree metadata
│   ├── main/                   # Worktree for main branch (directly in repo root)
│   ├── feature-x/              # Worktree for feature-x
│   └── bugfix-y/
├── project-b.git/
│   ├── main/
│   └── feature-z/
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
type Config struct {
    BaseDir        string  // Single directory for all bare repos
    // ... other existing fields unchanged (WorktreeFormat, Hooks, etc.)
}
```

**New config file format:**
```toml
# ~/.config/wt/config.toml
base_dir = "~/Git"              # All bare repos live here
                                # Worktrees are created directly in {repo}.git/{branch}
```

**Environment variables:**
- `WT_BASE_DIR` - Primary env var for base directory

**Tasks:**
- [ ] Replace `WorktreeDir`/`RepoDir` with `BaseDir` in Config struct
- [ ] Add `WT_BASE_DIR` env var support
- [ ] Update `config.ValidatePath()` for new field
- [ ] Add helper `Config.BareRepoPath(repoName)` → `{base_dir}/{repo}.git`
- [ ] Add helper `Config.WorktreePath(repoName, branch)` → `{base_dir}/{repo}.git/{branch}`

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
- When run from bare repo, returns worktrees directly in the bare repo directory
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
# ~/Git/project.git/main/      (worktree on default branch)

# Clone with custom name
wt clone github.com/user/project -n my-project
# Result: ~/Git/my-project.git/

# Clone bare only (no worktree)
wt clone github.com/user/project -N
# Result: ~/Git/project.git/ (no worktrees yet)
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
        worktreePath := filepath.Join(bareRepoPath, defaultBranch)

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
    // Worktree path is directly in the bare repo
    worktreePath := filepath.Join(repo.Path, sanitizeBranchName(branch))

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
- [ ] Update integration tests

---

#### 3.2 Update PR Checkout

**File:** `cmd/wt/pr.go`

Update `PrCheckoutCmd` to:
1. Clone as bare if repo doesn't exist
2. Create worktree directly in bare repo directory

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

    // Create worktree directly in bare repo
    worktreePath := filepath.Join(bareRepoPath, sanitizeBranchName(prInfo.Branch))

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

    // ... render table ...
}
```

**Tasks:**
- [ ] Update `ListCmd.Run()` for bare repo discovery
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
1. Scan for regular repos
2. For each repo:
   a. Move `.git` directory to become `<repo>.git` bare repo
   b. Configure bare repo settings
   c. Convert the original working directory into a worktree
   d. Move any existing worktrees into the bare repo directory
   e. Run `git worktree repair` to fix paths

**In-place conversion approach** (preserves existing checkout as worktree):
```bash
# Given: ~/Git/project/ (regular repo on branch "main")
# Result: ~/Git/project.git/ (bare repo)
#         ~/Git/project.git/main/ (worktree - former working dir)

# Step 1: Move .git to bare repo location
mv ~/Git/project/.git ~/Git/project.git

# Step 2: Configure as bare repo
git -C ~/Git/project.git config core.bare true

# Step 3: Set up fetch refspec (bare repos need this)
git -C ~/Git/project.git config remote.origin.fetch "+refs/heads/*:refs/remotes/origin/*"

# Step 4: Move working directory into bare repo as worktree
mv ~/Git/project ~/Git/project.git/main

# Step 5: Create .git file in worktree pointing to bare repo
echo "gitdir: ~/Git/project.git" > ~/Git/project.git/main/.git

# Step 6: Register worktree with git
git -C ~/Git/project.git worktree add --existing ~/Git/project.git/main main

# Step 7: Repair any path issues
git -C ~/Git/project.git worktree repair
```

**Migration script logic:**
```go
func (c *MigrateCmd) Run(ctx context.Context) error {
    cfg := c.Config
    l := log.FromContext(ctx)

    // Find all regular repos in base_dir
    repos, err := git.FindAllRepos(cfg.BaseDir)
    if err != nil {
        return err
    }

    for _, repo := range repos {
        repoName := filepath.Base(repo.Path)
        bareRepoPath := filepath.Join(cfg.BaseDir, repoName+".git")

        l.Info("Migrating", "repo", repoName)

        if c.DryRun {
            l.Info("Would convert to bare repo", "from", repo.Path, "to", bareRepoPath)
            continue
        }

        // Get current branch before conversion
        currentBranch, _ := git.CurrentBranch(ctx, repo.Path)
        if currentBranch == "" {
            currentBranch = "main"
        }

        // Step 1: Convert to bare in-place
        if err := c.convertToBareInPlace(ctx, repo.Path, bareRepoPath, currentBranch); err != nil {
            return fmt.Errorf("failed to convert %s: %w", repoName, err)
        }

        // Step 2: Move existing worktrees into bare repo
        worktrees, _ := git.ListWorktreesForRepo(repo.Path)
        for _, wt := range worktrees {
            newPath := filepath.Join(bareRepoPath, sanitizeBranchName(wt.Branch))

            if err := c.moveWorktree(ctx, wt.Path, newPath, bareRepoPath); err != nil {
                l.Warn("Failed to move worktree", "path", wt.Path, "error", err)
            }
        }

        // Step 3: Repair worktree links
        git.RepairWorktrees(ctx, bareRepoPath)
    }

    return nil
}

func (c *MigrateCmd) convertToBareInPlace(ctx context.Context, repoPath, bareRepoPath, branch string) error {
    gitDir := filepath.Join(repoPath, ".git")
    worktreePath := filepath.Join(bareRepoPath, sanitizeBranchName(branch))

    // Move .git to become bare repo
    if err := os.Rename(gitDir, bareRepoPath); err != nil {
        return fmt.Errorf("move .git: %w", err)
    }

    // Configure as bare
    cmd := exec.CommandContext(ctx, "git", "-C", bareRepoPath, "config", "core.bare", "true")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("set core.bare: %w", err)
    }

    // Set up fetch refspec
    cmd = exec.CommandContext(ctx, "git", "-C", bareRepoPath,
        "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
    cmd.Run() // Ignore error if no remote

    // Move working directory into bare repo
    if err := os.Rename(repoPath, worktreePath); err != nil {
        return fmt.Errorf("move working dir: %w", err)
    }

    // Create .git file in worktree
    gitFile := filepath.Join(worktreePath, ".git")
    content := fmt.Sprintf("gitdir: %s\n", bareRepoPath)
    if err := os.WriteFile(gitFile, []byte(content), 0644); err != nil {
        return fmt.Errorf("write .git file: %w", err)
    }

    // Register worktree with git (creates worktrees/<name> metadata)
    cmd = exec.CommandContext(ctx, "git", "-C", bareRepoPath,
        "worktree", "add", "--existing", worktreePath, branch)
    if err := cmd.Run(); err != nil {
        // Fallback: manually create worktree metadata
        return c.createWorktreeMetadata(bareRepoPath, worktreePath, branch)
    }

    return nil
}

func (c *MigrateCmd) moveWorktree(ctx context.Context, oldPath, newPath, bareRepoPath string) error {
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

### Phase 7: Documentation

#### 7.1 Documentation Updates

- [ ] Update README.md with new architecture
- [ ] Add migration guide section
- [ ] Update example config
- [ ] Document new `wt clone` command
- [ ] Document `wt migrate` command

**Tasks:**
- [ ] Create migration documentation
- [ ] Update README.md
- [ ] Update help text for affected commands

---

## Implementation Order

### Sprint 1: Foundation
1. Config changes (1.1) - Replace `worktree_dir`/`repo_dir` with `base_dir`
2. Bare repo detection (1.2)
3. Worktree discovery updates (1.3)

### Sprint 2: Core Commands
4. Clone command (2.1, 2.2)
5. Checkout updates (3.1)
6. PR checkout updates (3.2)

### Sprint 3: List & Discovery
7. List command updates (4.1)
8. Other command updates (5.1)

### Sprint 4: Migration & Polish
9. Move command rewrite (5.2)
10. Migration tool (6.1)
11. Documentation (7.1)
12. Final testing & edge cases

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
- Config parsing with `base_dir`

### Integration Tests
- Clone bare repo
- Create worktree in bare repo
- List worktrees from bare repo
- Migration from regular to bare (in-place conversion)

### Manual Testing Checklist
- [ ] Fresh install with new config
- [ ] Migration from existing setup
- [ ] GitHub workflow (clone, checkout PR, create PR, merge)
- [ ] GitLab workflow (same)
- [ ] Hooks work correctly
- [ ] Shell completions work
- [ ] `wt cd` works with new paths

---

## Design Decisions

1. **Worktree naming**: Use sanitized branch name directly in bare repo root
   - `project.git/feature-x` (branch `feature/x` becomes `feature-x`)
   - Clean and simple, repo context is the parent `.git` directory

2. **Default branch worktree**: Create by default on clone
   - `--no-worktree` / `-N` flag to skip initial worktree creation

3. **Bare repo suffix**: `.git` suffix required
   - Standard convention, makes intent clear
   - Easy to distinguish from worktree directories

4. **No backwards compatibility**: Clean break from old `worktree_dir`/`repo_dir` config
   - Users run `wt migrate` once to convert existing setup
   - Simpler codebase without legacy support
