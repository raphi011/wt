# wt v2 Architecture Plan

This document outlines a reworked architecture for `wt` that:
- Uses a registry as source of truth for repos (worktree data from git directly)
- Supports both bare and regular repos
- Allows flexible worktree placement (nested, sibling, or centralized)

---

## Current vs New Architecture

### Current Architecture
```
~/.config/wt/config.toml      # Config with repo_dir + worktree_dir
~/Git/                        # repo_dir (scanned for repos)
├── project-a/
├── project-b/
~/Git/worktrees/              # worktree_dir (flat list, scanned)
├── project-a-main/
├── project-a-feature/
├── project-b-main/
```

**Problems:**
- Directory scanning is slow and inflexible
- Two directories to configure
- Worktree location is fixed (all in one flat folder)
- No support for bare repos
- Can't have repos in arbitrary locations

### New Architecture (Registry-Based)
```
~/.wt/                        # Global wt state
├── config.toml               # Minimal config (defaults, hooks)
└── repos.json                # Registered repos (source of truth)

# Repos can live ANYWHERE - just register them:

~/work/project-a/             # Regular repo with nested worktrees
├── .git/
├── main/                     # worktree (worktree_dir: ".")
├── feature-x/
└── bugfix-y/

~/work/project-b.git/         # Bare repo with nested worktrees
├── HEAD, objects/, refs/
├── main/                     # worktree
└── feature-z/

~/oss/legacy/                 # Regular repo with sibling worktrees
├── .git/
~/oss/legacy-main/            # worktree (worktree_dir: "..")
~/oss/legacy-feature/

~/random/path/repo/           # Repo with centralized worktrees
├── .git/
~/worktrees/repo-main/        # worktree (worktree_dir: "~/worktrees")
~/worktrees/repo-feature/
```

**Benefits:**
- Repos can live anywhere - no directory restrictions
- Support bare and regular repos equally
- Flexible worktree placement per repo
- Worktree data always fresh (from `git worktree list`, no stale cache)
- Explicit control over what's tracked

---

## Core Concepts

### Repo Registry (`~/.wt/repos.json`)

The registry is the source of truth for repos `wt` manages:

```json
{
  "repos": [
    {
      "path": "/home/user/work/project-a",
      "name": "project-a",
      "worktree_format": "./{branch}",
      "labels": ["work"]
    },
    {
      "path": "/home/user/work/project-b.git",
      "name": "project-b",
      "labels": ["work"]
    },
    {
      "path": "/home/user/oss/legacy",
      "name": "legacy",
      "worktree_format": "../{repo}-{branch}",
      "labels": ["oss"]
    },
    {
      "path": "/home/user/random/repo",
      "name": "my-repo",
      "worktree_format": "~/worktrees/{repo}-{branch}",
      "labels": []
    }
  ]
}
```

Note: `worktree_format` is optional per-repo. If not set, uses the global `worktree_format` from config.

### Worktree Format String

The `worktree_format` config controls both **location** and **naming** of worktrees. It supports placeholders and can include relative or absolute paths:

**Placeholders:**
- `{repo}` - repository name
- `{branch}` - branch name (sanitized: `/` → `-`)

**Format Examples:**

| Format | Type | Example Output |
|--------|------|----------------|
| `{branch}` | Nested (default) | `project/main/`, `project/feature-x/` |
| `./{branch}` | Nested (explicit) | `project/main/`, `project/feature-x/` |
| `../{repo}-{branch}` | Sibling | `project-main/`, `project-feature-x/` |
| `~/worktrees/{repo}-{branch}` | Centralized | `~/worktrees/project-main/` |

**Path resolution:**
- Starts with `../` → relative to repo's parent directory
- Starts with `/` or `~/` → absolute path
- Everything else (including `./` prefix) → relative to repo path

**Resolution logic:**
```go
func ResolveWorktreePath(repo Repo, branch string, format string) string {
    // Apply placeholders
    path := strings.ReplaceAll(format, "{repo}", repo.Name)
    path = strings.ReplaceAll(path, "{branch}", sanitizeBranch(branch))

    switch {
    case strings.HasPrefix(path, "../"):
        // Sibling to repo: ../repo-branch → parent/repo-branch
        return filepath.Join(filepath.Dir(repo.Path), path[3:])

    case strings.HasPrefix(path, "~/"):
        // Home-relative absolute path
        return expandHome(path)

    case strings.HasPrefix(path, "/"):
        // Absolute path
        return path

    default:
        // Relative to repo (with or without ./ prefix)
        path = strings.TrimPrefix(path, "./")
        return filepath.Join(repo.Path, path)
    }
}
```

**Config examples:**

```toml
# ~/.wt/config.toml

# Nested worktrees inside repo (default)
worktree_format = "{branch}"

# Also nested (explicit ./ prefix)
worktree_format = "./{branch}"

# Sibling worktrees next to repo
worktree_format = "../{repo}-{branch}"

# Centralized folder (absolute path)
worktree_format = "~/worktrees/{repo}-{branch}"
```

**Per-repo override:**
Each repo in the registry can override the global format:

```json
{
  "repos": [
    {
      "path": "/home/user/work/project-a",
      "name": "project-a",
      "worktree_format": "./{branch}",
      "labels": ["work"]
    },
    {
      "path": "/home/user/oss/legacy",
      "name": "legacy",
      "worktree_format": "~/worktrees/{repo}-{branch}",
      "labels": ["oss"]
    }
  ]
}
```

### Repo Name Disambiguation

Since repos can be anywhere, names might collide. Strategy:
- Unique names: use short form (`project`)
- Duplicate names: require qualification or use labels

```bash
# If 'cmd' is registered twice
wt cd -r cmd
# Error: 'cmd' is ambiguous. Registered repos:
#   /home/user/work/cmd (labels: work)
#   /home/user/oss/cmd (labels: oss)
# Use: wt cd -r work/cmd or wt cd -l work -r cmd

# Using labels
wt cd -l work -r cmd      # uses label to disambiguate
wt list -l oss            # list only oss-labeled repos
```

---

## Implementation Phases

### Phase 1: Core Infrastructure

#### 1.1 Repo Registry

**File:** `internal/registry/registry.go` (new)

```go
type Repo struct {
    Path           string   `json:"path"`                      // Absolute path to repo
    Name           string   `json:"name"`                      // Display name
    WorktreeFormat string   `json:"worktree_format,omitempty"` // Optional override (e.g., "./{branch}")
    Labels         []string `json:"labels,omitempty"`          // Optional labels for grouping
}

type Registry struct {
    Repos []Repo `json:"repos"`
}

// Load reads registry from ~/.wt/repos.json
func Load() (*Registry, error)

// Save writes registry to ~/.wt/repos.json
func (r *Registry) Save() error

// Add registers a new repo
func (r *Registry) Add(repo Repo) error

// Remove unregisters a repo by name or path
func (r *Registry) Remove(nameOrPath string) error

// Find looks up a repo by name, path, or label+name
func (r *Registry) Find(ref string) (*Repo, error)

// FindByLabel returns repos matching a label
func (r *Registry) FindByLabel(label string) []Repo
```

**Tasks:**
- [ ] Create `internal/registry/registry.go`
- [ ] Implement `Load()`, `Save()`, `Add()`, `Remove()`, `Find()`
- [ ] Add label filtering
- [ ] Handle name disambiguation
- [ ] Add unit tests

---

#### 1.2 Config Updates

**File:** `internal/config/config.go`

Update config to support path-aware format strings:

```toml
# ~/.wt/config.toml

# Default worktree format (supports relative and absolute paths)
# "{branch}" or "./{branch}" = nested inside repo
# "../{repo}-{branch}" = sibling to repo
# "~/worktrees/{repo}-{branch}" = centralized folder
worktree_format = "{branch}"

# Default labels for new repos (optional)
default_labels = []

# Hooks (unchanged)
[[hooks]]
name = "post-checkout"
on = ["checkout"]
run = "echo checked out"
```

```go
type Config struct {
    WorktreeFormat string   `toml:"worktree_format"` // e.g., "{branch}", "~/worktrees/{repo}-{branch}"
    DefaultLabels  []string `toml:"default_labels"`
    Hooks          []Hook   `toml:"hooks"`
    // ... other existing fields
}

const DefaultWorktreeFormat = "{branch}"  // Default: nested worktrees
```

**Tasks:**
- [ ] Update `WorktreeFormat` default to `{branch}`
- [ ] Remove `WorktreeDir` and `RepoDir` from config
- [ ] Add path prefix detection to format resolution
- [ ] Move config to `~/.wt/config.toml`
- [ ] Add `DefaultLabels`
- [ ] Update config loading/validation

---

#### 1.3 Repo Detection Updates

**File:** `internal/git/repo.go`

Support both bare and regular repos:

```go
// RepoType indicates whether a repo is bare or regular
type RepoType int

const (
    RepoTypeRegular RepoType = iota
    RepoTypeBare
)

// DetectRepoType determines if a path is a bare or regular repo
func DetectRepoType(path string) (RepoType, error) {
    // Check for .git directory (regular repo)
    gitDir := filepath.Join(path, ".git")
    if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
        return RepoTypeRegular, nil
    }

    // Check for bare repo markers (HEAD file at root)
    headFile := filepath.Join(path, "HEAD")
    if _, err := os.Stat(headFile); err == nil {
        // Verify with git
        cmd := exec.Command("git", "-C", path, "rev-parse", "--is-bare-repository")
        out, err := cmd.Output()
        if err == nil && strings.TrimSpace(string(out)) == "true" {
            return RepoTypeBare, nil
        }
    }

    return 0, fmt.Errorf("not a git repository: %s", path)
}

// GetGitDir returns the git directory for a repo
func GetGitDir(repoPath string, repoType RepoType) string {
    if repoType == RepoTypeBare {
        return repoPath
    }
    return filepath.Join(repoPath, ".git")
}
```

**Tasks:**
- [ ] Add `DetectRepoType()`
- [ ] Add `GetGitDir()` helper
- [ ] Update `GetMainRepoPath()` to handle bare repos
- [ ] Add unit tests

---

#### 1.4 Worktree Path Resolution

**File:** `internal/format/worktree.go`

Update format resolution to support path-aware format strings:

```go
// ResolveWorktreePath computes the path for a new worktree
// format: e.g., "{branch}", "../{repo}-{branch}", "~/worktrees/{repo}-{branch}"
func ResolveWorktreePath(repo registry.Repo, branch string, cfg *config.Config) string {
    // Use repo-specific format or fall back to config default
    format := repo.WorktreeFormat
    if format == "" {
        format = cfg.WorktreeFormat
    }

    // Apply placeholders
    path := strings.ReplaceAll(format, "{repo}", repo.Name)
    path = strings.ReplaceAll(path, "{branch}", sanitizeBranch(branch))

    switch {
    case strings.HasPrefix(path, "../"):
        // Sibling to repo: ../repo-main → parent/repo-main
        return filepath.Join(filepath.Dir(repo.Path), path[3:])

    case strings.HasPrefix(path, "~/"):
        // Home-relative absolute path
        return expandHome(path)

    case strings.HasPrefix(path, "/"):
        // Absolute path
        return path

    default:
        // Relative to repo (with or without ./ prefix)
        path = strings.TrimPrefix(path, "./")
        return filepath.Join(repo.Path, path)
    }
}

// ListWorktreesForRepo lists all worktrees for a registered repo
func ListWorktreesForRepo(repo registry.Repo) ([]Worktree, error) {
    repoType, err := DetectRepoType(repo.Path)
    if err != nil {
        return nil, err
    }

    gitDir := GetGitDir(repo.Path, repoType)

    // Use git worktree list --porcelain
    cmd := exec.Command("git", "-C", gitDir, "worktree", "list", "--porcelain")
    // ... parse output
}
```

**Tasks:**
- [ ] Implement `ResolveWorktreePath()`
- [ ] Update `ListWorktreesForRepo()` to use registry
- [ ] Handle both bare and regular repos
- [ ] Add unit tests

---

### Phase 2: New Commands

#### 2.1 `wt add` - Register Existing Repo

**File:** `cmd/wt/add.go` (new)

```go
type AddCmd struct {
    Deps

    Path           string   `arg:"" help:"Path to git repository"`
    Name           string   `short:"n" help:"Display name (default: directory name)"`
    WorktreeFormat string   `short:"w" help:"Worktree format (default: from config)"`
    Labels         []string `short:"l" help:"Labels for grouping"`
}
```

**Usage:**
```bash
# Register existing repo (uses default format from config)
wt add ~/work/my-project

# Register with custom name and labels
wt add ~/code/repo -n my-repo -l work -l important

# Register with nested worktrees (branch-only names)
wt add ~/work/project -w "./{branch}"

# Register with sibling worktrees
wt add ~/oss/lib -w "../{repo}-{branch}"

# Register with centralized worktrees
wt add ~/random/repo -w "~/worktrees/{repo}-{branch}"
```

**Tasks:**
- [ ] Create `cmd/wt/add.go`
- [ ] Validate path is a git repo
- [ ] Detect repo type (bare/regular)
- [ ] Add to registry
- [ ] Update completions

---

#### 2.2 `wt clone` - Clone and Register

**File:** `cmd/wt/clone.go` (updated)

```go
type CloneCmd struct {
    Deps

    URL            string   `arg:"" help:"Repository URL"`
    Dest           string   `arg:"" optional:"" help:"Destination path"`
    Name           string   `short:"n" help:"Display name (default: from URL)"`
    Bare           bool     `short:"B" help:"Clone as bare repository"`
    WorktreeFormat string   `short:"w" help:"Worktree format (default: from config)"`
    Labels         []string `short:"l" help:"Labels for grouping"`
    NoWorktree     bool     `short:"N" help:"Don't create initial worktree"`
}
```

**Usage:**
```bash
# Clone regular repo with nested worktrees (default: ./{branch})
wt clone github.com/user/project ~/work/project
# Result: ~/work/project/.git/ + ~/work/project/main/

# Clone as bare repo
wt clone github.com/user/project ~/work/project.git --bare
# Result: ~/work/project.git/ (bare) + ~/work/project.git/main/

# Clone with sibling worktrees
wt clone github.com/user/project ~/work/project -w "../{repo}-{branch}"
# Result: ~/work/project/.git/ + ~/work/project-main/

# Clone with centralized worktrees
wt clone github.com/user/project ~/work/project -w "~/worktrees/{repo}-{branch}"
# Result: ~/work/project/.git/ + ~/worktrees/project-main/
```

**Tasks:**
- [ ] Update `cmd/wt/clone.go`
- [ ] Support `--bare` flag
- [ ] Register cloned repo in registry
- [ ] Create initial worktree based on `worktree_format`
- [ ] Update completions

---

#### 2.3 `wt remove` - Unregister Repo

**File:** `cmd/wt/remove.go` (new)

```go
type RemoveCmd struct {
    Deps

    Repo   string `arg:"" help:"Repo name or path"`
    Delete bool   `short:"D" help:"Also delete repo and worktrees from disk"`
    Force  bool   `short:"f" help:"Force deletion without confirmation"`
}
```

**Usage:**
```bash
# Unregister (keep files)
wt remove my-project

# Unregister and delete
wt remove my-project --delete
```

**Tasks:**
- [ ] Create `cmd/wt/remove.go`
- [ ] Remove from registry
- [ ] Optional deletion with confirmation
- [ ] Update completions

---

#### 2.4 `wt repos` - List Registered Repos

**File:** `cmd/wt/repos.go` (new)

```go
type ReposCmd struct {
    Deps

    Label string `short:"l" help:"Filter by label"`
    JSON  bool   `long:"json" help:"Output as JSON"`
}
```

**Usage:**
```bash
wt repos
# NAME        PATH                      TYPE    WORKTREE_DIR  LABELS
# project-a   ~/work/project-a          regular .             work
# project-b   ~/work/project-b.git      bare    .             work
# legacy      ~/oss/legacy              regular ..            oss

wt repos -l work
# (filtered by label)
```

**Tasks:**
- [ ] Create `cmd/wt/repos.go`
- [ ] Table output with repo info
- [ ] Label filtering
- [ ] JSON output
- [ ] Update completions

---

### Phase 3: Update Existing Commands

#### 3.1 `wt list` - Use Registry

Update to read from registry instead of scanning:

```go
func (c *ListCmd) Run(ctx context.Context) error {
    reg, err := registry.Load()
    if err != nil {
        return err
    }

    var allWorktrees []git.Worktree

    for _, repo := range reg.Repos {
        // Filter by label if specified
        if c.Label != "" && !containsLabel(repo.Labels, c.Label) {
            continue
        }

        worktrees, err := git.ListWorktreesForRepo(repo)
        if err != nil {
            continue
        }

        for i := range worktrees {
            worktrees[i].RepoName = repo.Name
            worktrees[i].Labels = repo.Labels
        }

        allWorktrees = append(allWorktrees, worktrees...)
    }

    // ... render table
}
```

**Tasks:**
- [ ] Update `wt list` to use registry
- [ ] Add `-l` label filter
- [ ] Show repo name from registry
- [ ] Update integration tests

---

#### 3.2 `wt checkout` - Use Registry

Update to resolve repo from registry:

```go
func (c *CheckoutCmd) Run(ctx context.Context) error {
    reg, err := registry.Load()

    // Find repo (from -r flag, current directory, or interactive)
    repo, err := c.resolveRepo(reg)

    // Compute worktree path using repo's worktree_dir
    wtPath := git.ResolveWorktreePath(repo, c.Branch)

    // Create worktree
    gitDir := git.GetGitDir(repo.Path, repoType)
    return git.AddWorktree(ctx, gitDir, wtPath, c.Branch)
}
```

**Tasks:**
- [ ] Update `wt checkout` to use registry
- [ ] Use `ResolveWorktreePath()` for worktree location
- [ ] Handle bare repos
- [ ] Update integration tests

---

#### 3.3 Other Commands

| Command | Changes |
|---------|---------|
| `wt cd` | Use registry to find repo/worktree |
| `wt exec` | Use registry |
| `wt prune` | Use registry, handle different worktree locations |
| `wt pr checkout` | Clone and register if new repo |
| `wt hook` | Use registry |
| `wt note` | Use registry |
| `wt mv` | **Remove** - no longer needed with registry model |
| `wt doctor` | Updated checks for registry model |

**`wt doctor` updated checks:**
- Registry file is valid JSON
- Registered repos exist on disk
- Registered repos are valid git repos (bare or regular)
- Worktrees are valid (no broken/orphaned)
- Config file is valid TOML
- External tools installed (git, gh/glab)
- No duplicate repo names without labels
- Optionally: offer to remove stale registry entries

**Why remove `wt mv`?**

The old `wt mv` renamed worktrees when `worktree_format` changed. With the registry model:
- To move a repo: `mv` the directory, then update registry with `wt remove` + `wt add`
- Worktree locations are determined by `worktree_format` at creation time
- Existing worktrees stay where they are (git tracks them by path)
- Changing `worktree_format` only affects new worktrees

---

### Phase 4: Migration

#### 4.1 `wt migrate` - Import Existing Setup

For users migrating from current wt:

```go
type MigrateCmd struct {
    Deps

    RepoDir     string `arg:"" help:"Current repo_dir to scan"`
    WorktreeDir string `help:"Current worktree_dir (optional)"`
    DryRun      bool   `short:"d" help:"Show what would be imported"`
}
```

**Usage:**
```bash
# Import existing repos
wt migrate ~/Git

# With separate worktree dir
wt migrate ~/Git --worktree-dir ~/Git/worktrees

# Preview
wt migrate ~/Git -d
```

This scans the old directories and registers found repos.

**Tasks:**
- [ ] Create migration command
- [ ] Scan for repos
- [ ] Detect worktree associations
- [ ] Register in new format
- [ ] Dry-run mode

---

## Config File

### New Format

```toml
# ~/.wt/config.toml

# Default worktree format (supports path prefixes + placeholders)
# "{branch}" or "./{branch}" = nested inside repo
# "../{repo}-{branch}" = sibling to repo
# "~/worktrees/{repo}-{branch}" = centralized folder
worktree_format = "{branch}"

# Default labels applied to new repos
default_labels = []

# Hooks (unchanged from current)
[[hooks]]
name = "post-checkout"
on = ["checkout"]
run = "make install"

[[hooks]]
name = "lint"
on = ["checkout", "pr-checkout"]
run = "./scripts/lint.sh"
```

---

## Breaking Changes

| Change | Impact | Migration |
|--------|--------|-----------|
| Config format | `repo_dir`/`worktree_dir` removed | Update config, run `wt migrate` |
| Config location | `~/.config/wt/` → `~/.wt/` | Auto-detected or manual move |
| Repo tracking | Scan → Registry | Run `wt migrate` or `wt add` |
| Default worktree location | Nested in repo (was flat folder) | Update `worktree_format` if needed |

---

## Design Decisions

1. **Registry as source of truth**
   - No filesystem scanning
   - Explicit control over tracked repos
   - Repos can live anywhere

2. **Support both bare and regular repos**
   - No enforcement of structure
   - Auto-detect repo type
   - Same commands work for both

3. **Unified worktree_format string**
   - Single config field for location + naming
   - Path prefix determines resolution: `../` (sibling), `~/` or `/` (absolute)
   - No prefix or `./` = relative to repo (nested)
   - Placeholders: `{repo}`, `{branch}`
   - Per-repo override possible

4. **Labels for organization**
   - Stored in registry (`repos.json`), not in git config
   - Optional grouping mechanism
   - Filter commands by label
   - Helps with disambiguation

5. **Global state in `~/.wt/`**
   - `config.toml` - user preferences
   - `repos.json` - registered repos
   - No worktree cache - always use `git worktree list` for fresh data

6. **Migration path provided**
   - `wt migrate` imports existing repos from old setup
   - `wt add` registers individual repos
   - Existing worktrees remain on disk (git doesn't care where they are)

---

## Implementation Order

### Sprint 1: Foundation
1. Registry implementation (1.1)
2. Config simplification (1.2)
3. Repo type detection (1.3)
4. Worktree path resolution (1.4)

### Sprint 2: New Commands
5. `wt add` command (2.1)
6. `wt clone` updates (2.2)
7. `wt remove` command (2.3)
8. `wt repos` command (2.4)

### Sprint 3: Update Existing
9. `wt list` updates (3.1)
10. `wt checkout` updates (3.2)
11. Other command updates (3.3)

### Sprint 4: Polish
12. Migration command (4.1)
13. Documentation
14. Integration tests

---

## Testing Strategy

### Unit Tests
- Registry CRUD operations
- Repo type detection (bare vs regular)
- Worktree path resolution (all modes)
- Name disambiguation

### Integration Tests
- `wt add` with various worktree_dir modes
- `wt clone` bare and regular
- `wt list` from registry
- `wt checkout` with nested/sibling/centralized worktrees
- Migration from old format

### Manual Testing
- [ ] Fresh install
- [ ] Migration from existing setup
- [ ] Mixed bare/regular repos
- [ ] Different worktree_dir modes
- [ ] Label filtering
- [ ] GitHub/GitLab workflows
