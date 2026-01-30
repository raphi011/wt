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
~/.wt/                          # Global config & cache
├── config.toml                 # Configuration file
├── cache.json                  # Worktree cache (all base_dirs)
└── cache.lock                  # Cache lock file

~/work/                         # First base_dir (default for new clones)
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

~/personal/                     # Second base_dir
├── side-project.git/
│   ├── main/
│   └── experiment/

~/oss/                          # Third base_dir
├── open-source-lib.git/
│   └── main/
```

**Benefits:**
- Multiple directories supported (work, personal, oss, etc.)
- First base_dir is default for new clones
- All repos treated equally regardless of location
- Global config/cache in `~/.wt/` - single source of truth
- Clear project grouping within each base_dir
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
    BaseDirs       []string  // List of directories containing bare repos
                             // First entry is default for new clones
    // ... other existing fields unchanged (WorktreeFormat, Hooks, etc.)
}
```

**Global config location:** `~/.wt/config.toml`

This replaces the previous `~/.config/wt/config.toml` location. All wt state lives in `~/.wt/`:
- `~/.wt/config.toml` - Configuration file
- `~/.wt/cache.json` - Worktree cache (spans all base_dirs)
- `~/.wt/cache.lock` - Cache lock file

**New config file format:**
```toml
# ~/.wt/config.toml
base_dirs = [
    "~/work",      # First = default for new clones
    "~/personal",
    "~/oss",
]

# Worktrees are created directly in {base_dir}/{repo}.git/{branch}
```

**Environment variables:**
- `WT_BASE_DIRS` - Comma-separated list of base directories (overrides config)
- `WT_CONFIG_DIR` - Override config directory (default: `~/.wt`)

**Behavior:**
- `wt clone` uses first base_dir as destination
- `wt clone --base-dir ~/oss` can override for specific clone
- `wt list` scans all base_dirs
- Cache stores worktrees from all base_dirs with unique IDs
- All repos are treated equally regardless of which base_dir they're in

**Tasks:**
- [ ] Replace `WorktreeDir`/`RepoDir` with `BaseDirs` in Config struct
- [ ] Move config location to `~/.wt/config.toml`
- [ ] Move cache to `~/.wt/cache.json`
- [ ] Add `WT_BASE_DIRS` env var support (comma-separated)
- [ ] Add `WT_CONFIG_DIR` env var support
- [ ] Update `config.ValidatePath()` for new fields
- [ ] Add helper `Config.DefaultBaseDir()` → first entry in BaseDirs
- [ ] Add helper `Config.BareRepoPath(repoName)` → `{default_base_dir}/{repo}.git`
- [ ] Add helper `Config.WorktreePath(repoName, branch)` → `{base_dir}/{repo}.git/{branch}`
- [ ] Add helper `Config.FindRepo(repoName)` → searches all base_dirs

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

#### 1.3 Repo Name Disambiguation

With multiple `base_dirs`, the same repo name can exist in different directories:
```
~/work/cmd.git/       # work project
~/oss/cmd.git/        # open source project
```

**Naming strategy:**
- If repo name is **unique** across all base_dirs: use short name (`cmd`)
- If repo name has **duplicates**: use qualified name (`work/cmd`, `oss/cmd`)

**Implementation:**

```go
// RepoRef represents a repository reference that may be qualified
type RepoRef struct {
    BaseDir  string  // e.g., "~/work" or "" if unique
    Name     string  // e.g., "cmd"
    FullPath string  // e.g., "~/work/cmd.git"
}

// DisplayName returns the name to show in UI and accept as input
func (r RepoRef) DisplayName() string {
    if r.BaseDir == "" {
        return r.Name  // Unique, use short name
    }
    // Qualified: use base_dir basename + repo name
    return filepath.Base(r.BaseDir) + "/" + r.Name
}

// BuildRepoIndex scans all base_dirs and determines qualified names
func BuildRepoIndex(baseDirs []string) (map[string]RepoRef, error) {
    // First pass: collect all repos
    repos := make(map[string][]RepoRef)  // name -> list of refs

    for _, baseDir := range baseDirs {
        bareRepos, _ := git.FindAllBareRepos(baseDir)
        for _, repoPath := range bareRepos {
            name := git.GetBareRepoName(repoPath)
            repos[name] = append(repos[name], RepoRef{
                BaseDir:  baseDir,
                Name:     name,
                FullPath: repoPath,
            })
        }
    }

    // Second pass: determine display names
    index := make(map[string]RepoRef)
    for name, refs := range repos {
        if len(refs) == 1 {
            // Unique - use short name
            refs[0].BaseDir = ""  // Clear to indicate unique
            index[name] = refs[0]
        } else {
            // Duplicates - use qualified names
            for _, ref := range refs {
                displayName := ref.DisplayName()
                index[displayName] = ref
            }
        }
    }

    return index, nil
}
```

**Usage in commands:**
```bash
# If 'cmd' is unique across all base_dirs
wt cd -r cmd
wt checkout -r cmd feature-branch

# If 'cmd' exists in multiple base_dirs
wt cd -r work/cmd
wt cd -r oss/cmd
wt checkout -r work/cmd feature-branch

# List shows qualified names only when needed
wt list
#  ID  REPO       BRANCH    STATUS
#  1   project    main      clean     # unique
#  2   work/cmd   main      1 ahead   # qualified (duplicate)
#  3   oss/cmd    main      clean     # qualified (duplicate)
```

**Error handling:**
```bash
# Ambiguous reference
wt cd -r cmd
# Error: 'cmd' is ambiguous. Did you mean:
#   work/cmd
#   oss/cmd
```

**Tasks:**
- [ ] Create `RepoRef` struct
- [ ] Implement `BuildRepoIndex()`
- [ ] Update `-r` flag handling to resolve qualified names
- [ ] Update list output to show qualified names when needed
- [ ] Add error message for ambiguous references
- [ ] Add unit tests for disambiguation

---

#### 1.4 Worktree Discovery Updates

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
    BaseDir     string `short:"b" help:"Target base directory (default: first in base_dirs)"`
    NoWorktree  bool   `short:"N" help:"Clone bare repo only, don't create initial worktree"`
}
```

**Behavior:**
```bash
# Clone as bare repo + create main worktree (uses first base_dir)
wt clone github.com/user/project
# Result:
# ~/work/project.git/           (bare repo in default base_dir)
# ~/work/project.git/main/      (worktree on default branch)

# Clone with custom name
wt clone github.com/user/project -n my-project
# Result: ~/work/my-project.git/

# Clone to specific base_dir
wt clone github.com/user/project -b ~/oss
# Result: ~/oss/project.git/

# Clone bare only (no worktree)
wt clone github.com/user/project -N
# Result: ~/work/project.git/ (no worktrees yet)
```

**Implementation:**
```go
func (c *CloneCmd) Run(ctx context.Context) error {
    cfg := c.Config
    repoName := c.Name
    if repoName == "" {
        repoName = extractRepoName(c.RepoURL)
    }

    // Use specified base_dir or default (first in list)
    baseDir := c.BaseDir
    if baseDir == "" {
        baseDir = cfg.DefaultBaseDir()
    }

    bareRepoPath := filepath.Join(baseDir, repoName+".git")

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

Update `ListCmd` to discover bare repos across all base_dirs:

```go
func (c *ListCmd) Run(ctx context.Context) error {
    cfg := c.Config

    var allWorktrees []git.Worktree

    // Scan all base_dirs for bare repos
    for _, baseDir := range cfg.BaseDirs {
        bareRepos, err := git.FindAllBareRepos(baseDir)
        if err != nil {
            continue // Skip inaccessible directories
        }

        for _, bareRepo := range bareRepos {
            worktrees, err := git.ListWorktreesForBareRepo(bareRepo)
            if err != nil {
                continue
            }
            allWorktrees = append(allWorktrees, worktrees...)
        }
    }

    // ... render table ...
}
```

**Worktree struct update:**
```go
type Worktree struct {
    // ... existing fields ...
    BaseDir     string // Which base_dir this worktree's repo is in
    RepoName    string // Short repo name (e.g., "cmd")
    DisplayName string // Qualified name if needed (e.g., "work/cmd" or "cmd")
}
```

The `DisplayName` is computed using `RepoRef.DisplayName()` logic - qualified only when duplicates exist.

**List output example:**
```
ID  REPO       BRANCH    STATUS
1   project    main      clean       # unique repo
2   work/cmd   main      1 ahead     # qualified (cmd exists in both work/ and oss/)
3   work/cmd   feature   2 behind
4   oss/cmd    main      clean
```

**Tasks:**
- [ ] Update `ListCmd.Run()` to scan all base_dirs
- [ ] Add `BaseDir`, `RepoName`, `DisplayName` fields to `Worktree` struct
- [ ] Use `BuildRepoIndex()` to determine display names
- [ ] Update table rendering to use `DisplayName`
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

    Source  string `arg:"" help:"Source directory to migrate (repo_dir or worktree_dir)"`
    Target  string `short:"t" help:"Target base_dir (default: first in base_dirs)"`
    DryRun  bool   `short:"d" help:"Show what would be migrated without making changes"`
}
```

**Usage:**
```bash
# Migrate repos from old repo_dir to first base_dir
wt migrate ~/Git

# Migrate to specific base_dir
wt migrate ~/Git --target ~/work

# Dry run to preview
wt migrate ~/Git -d
```

**Migration approach: In-place move**

Instead of cloning and patching, we move the `.git` directory directly and convert it to bare. This:
- Preserves all uncommitted changes (staged, unstaged, untracked)
- Preserves binary files without patches
- Preserves worktree metadata (just needs path updates)
- Is simpler and more robust

**In-place migration steps:**
1. Move `.git/` directory to become bare repo: `project/.git` → `project.git/`
2. Set `core.bare = true`
3. Move main repo into bare repo as worktree: `project/` → `project.git/main/`
4. Move existing worktrees into bare repo: `project-feature/` → `project.git/feature/`
5. Update worktree path references (`.git` files and `gitdir` files)
6. Create worktree metadata for main repo (it wasn't a worktree before)
7. Run `git worktree repair` to validate

**Shell script example:**

```bash
# Given:
# ~/Git/project/                    (main repo on "main" branch)
# ~/Git/worktrees/project-feature/  (worktree on "feature" branch)
# Both may have uncommitted changes (staged, unstaged, untracked)

# Step 1: Move .git to become bare repo (worktree metadata comes along!)
mv ~/Git/project/.git ~/Git/project.git
git -C ~/Git/project.git config core.bare true

# Step 2: Move directories into bare repo
mv ~/Git/project ~/Git/project.git/main
mv ~/Git/worktrees/project-feature ~/Git/project.git/feature

# Step 3: Update paths for existing worktrees (2 files each)
# The .git file in the worktree points to the metadata location
echo "gitdir: ~/Git/project.git/worktrees/project-feature" > ~/Git/project.git/feature/.git
# The gitdir file in metadata points back to the worktree
echo "~/Git/project.git/feature/.git" > ~/Git/project.git/worktrees/project-feature/gitdir

# Step 4: Create metadata for main repo (wasn't a worktree before - needs 5 files)
mkdir -p ~/Git/project.git/worktrees/main
echo "~/Git/project.git/main/.git" > ~/Git/project.git/worktrees/main/gitdir
echo "ref: refs/heads/main" > ~/Git/project.git/worktrees/main/HEAD
echo "../.." > ~/Git/project.git/worktrees/main/commondir
cp ~/Git/project.git/index ~/Git/project.git/worktrees/main/index
echo "gitdir: ~/Git/project.git/worktrees/main" > ~/Git/project.git/main/.git

# Step 5: Repair validates everything and fixes any path issues
git -C ~/Git/project.git worktree repair
```

**What this preserves (without patches!):**
- ✅ Unstaged changes (files move with directory)
- ✅ Staged changes (index moves with `.git`)
- ✅ Untracked files (files move with directory)
- ✅ Binary files (no patch limitations)
- ✅ All branches and commits
- ✅ Stashes (stored in `.git/refs/stash`)

**Worktree metadata files:**

For existing worktrees, update 2 files:
- `{bare_repo}/worktrees/{name}/gitdir` - absolute path to worktree's `.git` file
- `{worktree}/.git` - text file containing `gitdir: {path_to_metadata}`

For the main repo (converting to worktree), create 5 files:
- `{bare_repo}/worktrees/{name}/gitdir` - path to worktree's `.git` file
- `{bare_repo}/worktrees/{name}/HEAD` - `ref: refs/heads/{branch}`
- `{bare_repo}/worktrees/{name}/commondir` - relative path to bare repo (usually `../..`)
- `{bare_repo}/worktrees/{name}/index` - copy from `{bare_repo}/index`
- `{worktree}/.git` - text file containing `gitdir: {path_to_metadata}`

**Migration script logic:**
```go
func (c *MigrateCmd) Run(ctx context.Context) error {
    cfg := c.Config
    l := log.FromContext(ctx)

    // Find all regular repos (non-bare)
    repos, err := git.FindAllRepos(cfg.BaseDir)
    if err != nil {
        return err
    }

    for _, repo := range repos {
        repoName := filepath.Base(repo.Path)
        bareRepoPath := filepath.Join(cfg.BaseDir, repoName+".git")

        // Get all worktrees (includes main repo)
        worktrees, _ := git.ListWorktreesForRepo(repo.Path)

        // Find the main repo entry and existing worktrees
        var mainWorktree *git.Worktree
        var existingWorktrees []git.Worktree
        for _, wt := range worktrees {
            if wt.Path == repo.Path {
                mainWorktree = &wt
            } else {
                existingWorktrees = append(existingWorktrees, wt)
            }
        }

        l.Info("Migrating", "repo", repoName, "worktrees", len(worktrees))

        if c.DryRun {
            l.Info("Would move .git to bare repo", "from", repo.Path+"/.git", "to", bareRepoPath)
            l.Info("Would move main repo", "from", repo.Path, "to", filepath.Join(bareRepoPath, mainWorktree.Branch))
            for _, wt := range existingWorktrees {
                newPath := filepath.Join(bareRepoPath, sanitizeBranchName(wt.Branch))
                l.Info("Would move worktree", "from", wt.Path, "to", newPath)
            }
            continue
        }

        // Step 1: Move .git to become bare repo
        gitDir := filepath.Join(repo.Path, ".git")
        if err := os.Rename(gitDir, bareRepoPath); err != nil {
            return fmt.Errorf("move .git to bare repo: %w", err)
        }

        // Step 2: Set core.bare = true
        if err := git.SetConfig(ctx, bareRepoPath, "core.bare", "true"); err != nil {
            return fmt.Errorf("set core.bare: %w", err)
        }

        // Step 3: Move main repo into bare repo
        mainNewPath := filepath.Join(bareRepoPath, sanitizeBranchName(mainWorktree.Branch))
        if err := os.Rename(repo.Path, mainNewPath); err != nil {
            return fmt.Errorf("move main repo: %w", err)
        }

        // Step 4: Create worktree metadata for main repo
        if err := c.createWorktreeMetadata(bareRepoPath, mainNewPath, mainWorktree.Branch); err != nil {
            return fmt.Errorf("create main worktree metadata: %w", err)
        }

        // Step 5: Move existing worktrees and update paths
        for _, wt := range existingWorktrees {
            newPath := filepath.Join(bareRepoPath, sanitizeBranchName(wt.Branch))

            if err := os.Rename(wt.Path, newPath); err != nil {
                l.Warn("Failed to move worktree", "from", wt.Path, "error", err)
                continue
            }

            if err := c.updateWorktreePaths(bareRepoPath, newPath, wt.Branch); err != nil {
                l.Warn("Failed to update worktree paths", "branch", wt.Branch, "error", err)
            }
        }

        // Step 6: Repair validates everything
        if err := git.WorktreeRepair(ctx, bareRepoPath); err != nil {
            l.Warn("Worktree repair reported issues", "error", err)
        }

        l.Info("Migration complete", "repo", repoName)
    }

    return nil
}

// createWorktreeMetadata creates the 5 metadata files needed to convert the
// main repo into a worktree
func (c *MigrateCmd) createWorktreeMetadata(bareRepoPath, worktreePath, branch string) error {
    wtName := sanitizeBranchName(branch)
    metadataDir := filepath.Join(bareRepoPath, "worktrees", wtName)

    if err := os.MkdirAll(metadataDir, 0755); err != nil {
        return err
    }

    gitFilePath := filepath.Join(worktreePath, ".git")

    // 1. gitdir - points to worktree's .git file
    if err := os.WriteFile(
        filepath.Join(metadataDir, "gitdir"),
        []byte(gitFilePath+"\n"),
        0644,
    ); err != nil {
        return err
    }

    // 2. HEAD - ref to branch
    if err := os.WriteFile(
        filepath.Join(metadataDir, "HEAD"),
        []byte("ref: refs/heads/"+branch+"\n"),
        0644,
    ); err != nil {
        return err
    }

    // 3. commondir - relative path to bare repo
    if err := os.WriteFile(
        filepath.Join(metadataDir, "commondir"),
        []byte("../..\n"),
        0644,
    ); err != nil {
        return err
    }

    // 4. index - copy from bare repo
    srcIndex := filepath.Join(bareRepoPath, "index")
    dstIndex := filepath.Join(metadataDir, "index")
    if _, err := os.Stat(srcIndex); err == nil {
        if err := copyFile(srcIndex, dstIndex); err != nil {
            return err
        }
    }

    // 5. .git file in worktree - points to metadata
    if err := os.WriteFile(
        gitFilePath,
        []byte("gitdir: "+metadataDir+"\n"),
        0644,
    ); err != nil {
        return err
    }

    return nil
}

// updateWorktreePaths updates the 2 path files for an existing worktree
func (c *MigrateCmd) updateWorktreePaths(bareRepoPath, worktreePath, branch string) error {
    // Find the worktree metadata directory name (may not match sanitized branch)
    wtMetadataDir := filepath.Join(bareRepoPath, "worktrees")
    entries, err := os.ReadDir(wtMetadataDir)
    if err != nil {
        return err
    }

    // Find matching metadata by reading gitdir files
    var metadataPath string
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        gitdirPath := filepath.Join(wtMetadataDir, entry.Name(), "gitdir")
        // Check if this metadata matches our worktree (by old path)
        // We'll update it to the new path
        metadataPath = filepath.Join(wtMetadataDir, entry.Name())
        // For simplicity, assume the metadata dir name matches the branch pattern
        // A more robust implementation would check HEAD file
    }

    if metadataPath == "" {
        return fmt.Errorf("worktree metadata not found for branch %s", branch)
    }

    gitFilePath := filepath.Join(worktreePath, ".git")

    // 1. Update gitdir in metadata
    if err := os.WriteFile(
        filepath.Join(metadataPath, "gitdir"),
        []byte(gitFilePath+"\n"),
        0644,
    ); err != nil {
        return err
    }

    // 2. Update .git file in worktree
    if err := os.WriteFile(
        gitFilePath,
        []byte("gitdir: "+metadataPath+"\n"),
        0644,
    ); err != nil {
        return err
    }

    return nil
}
```

**Helper functions needed:**
```go
// git.SetConfig(ctx, repoPath, key, value) - runs: git -C <repo> config <key> <value>
// git.WorktreeRepair(ctx, repoPath) - runs: git -C <repo> worktree repair
// copyFile(src, dst) - standard file copy utility
```

**Tasks:**
- [ ] Create `cmd/wt/migrate.go`
- [ ] Add `git.SetConfig()` helper
- [ ] Add `git.WorktreeRepair()` helper
- [ ] Add `copyFile()` utility
- [ ] Add dry-run mode
- [ ] Add progress output
- [ ] Handle edge cases (worktree name collisions, permission errors)
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
1. Config changes (1.1) - Replace `worktree_dir`/`repo_dir` with `base_dirs`, move to `~/.wt/`
2. Bare repo detection (1.2)
3. Repo name disambiguation (1.3) - RepoRef, BuildRepoIndex, qualified names
4. Worktree discovery updates (1.4)

### Sprint 2: Core Commands
5. Clone command (2.1, 2.2)
6. Checkout updates (3.1)
7. PR checkout updates (3.2)

### Sprint 3: List & Discovery
8. List command updates (4.1) - scan all base_dirs, use display names
9. Other command updates (5.1) - update `-r` flag handling

### Sprint 4: Migration & Polish
9. Move command rewrite (5.2)
10. Migration tool (6.1)
11. Documentation (7.1)
12. Final testing & edge cases

---

## Breaking Changes

| Change | Impact | Migration |
|--------|--------|-----------|
| New directory structure | All worktrees move into bare repos | `wt migrate` command |
| Config location change | `~/.config/wt/` → `~/.wt/` | Manual move or recreate |
| Config format change | `repo_dir`/`worktree_dir` → `base_dirs` | Update config file |
| Cache location change | Per-directory → `~/.wt/cache.json` | Automatic rebuild |
| Worktree paths change | Hardcoded paths break | Use `wt cd` instead |
| Cache invalidation | IDs may change | Cache rebuilt automatically |

---

## Testing Strategy

### Unit Tests
- Bare repo detection
- Path sanitization
- Config parsing with `base_dirs` (list)
- Multi-directory scanning
- Repo name disambiguation (`BuildRepoIndex`)
- Display name generation (unique vs qualified)

### Integration Tests
- Clone bare repo to default base_dir
- Clone bare repo to specific base_dir (`-b`)
- Create worktree in bare repo
- List worktrees across multiple base_dirs
- Disambiguation: unique repo names use short form
- Disambiguation: duplicate repo names use qualified form
- Disambiguation: ambiguous `-r` reference shows error with suggestions
- Migration from regular to bare (in-place conversion)

### Manual Testing Checklist
- [ ] Fresh install with new config at `~/.wt/`
- [ ] Migration from existing setup
- [ ] Multiple base_dirs scanning
- [ ] Clone to non-default base_dir
- [ ] GitHub workflow (clone, checkout PR, create PR, merge)
- [ ] GitLab workflow (same)
- [ ] Hooks work correctly
- [ ] Shell completions work
- [ ] `wt cd` works with new paths

---

## Design Decisions

1. **Multiple base directories**: Support list of `base_dirs`
   - First entry is default for new clones
   - All directories scanned equally for `wt list`
   - Allows organizing by purpose (work, personal, oss)

2. **Repo name disambiguation**: Qualified names when duplicates exist
   - Unique repos use short name: `cmd`
   - Duplicate repos use qualified name: `work/cmd`, `oss/cmd`
   - Ambiguous `-r` references show helpful error with suggestions
   - Display names computed dynamically based on current repo index

3. **Global config location**: `~/.wt/` directory
   - Single location for config, cache, and lock files
   - Not tied to any specific base_dir
   - Simpler than XDG paths for cross-platform support

4. **Worktree naming**: Use sanitized branch name directly in bare repo root
   - `project.git/feature-x` (branch `feature/x` becomes `feature-x`)
   - Clean and simple, repo context is the parent `.git` directory

5. **Default branch worktree**: Create by default on clone
   - `--no-worktree` / `-N` flag to skip initial worktree creation

6. **Bare repo suffix**: `.git` suffix required
   - Standard convention, makes intent clear
   - Easy to distinguish from worktree directories

7. **No backwards compatibility**: Clean break from old `worktree_dir`/`repo_dir` config
   - Users run `wt migrate` once to convert existing setup
   - Simpler codebase without legacy support
