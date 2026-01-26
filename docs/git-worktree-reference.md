# Git Worktree Reference

This document covers git and worktree concepts used in the `wt` codebase. It serves as a reference for developers and AI agents working on this project.

## Table of Contents

- [Core Architecture](#core-architecture)
- [Worktree vs Main Repo Detection](#worktree-vs-main-repo-detection)
- [Worktree Lifecycle](#worktree-lifecycle)
- [Branch and Merge Status](#branch-and-merge-status)
- [The .git File and gitdir Links](#the-git-file-and-gitdir-links)
- [Repository Discovery](#repository-discovery)
- [Metadata Storage](#metadata-storage)
- [Performance Patterns](#performance-patterns)

---

## Core Architecture

All git operations use `os/exec.Command` to invoke the git CLI directly rather than Go libraries:

```go
// internal/cmd/exec.go
cmd := exec.CommandContext(ctx, "git", args...)
cmd.Dir = workingDir
output, err := cmd.Output()
```

**Why CLI over libraries:**
- User's SSH keys, credential helpers, and git aliases work correctly
- Changes isolated to `internal/git/` package
- No dependency on git Go libraries that may lag behind git features

**Wrapper functions in `internal/git/exec.go`:**
- `runGit(ctx, dir, args...)` - executes git command silently
- `outputGit(ctx, dir, args...)` - executes git and returns stdout

---

## Worktree vs Main Repo Detection

Git uses different `.git` formats to distinguish main repos from worktrees:

| Type | `.git` format | Example |
|------|---------------|---------|
| Main repo | Directory | `.git/` containing `HEAD`, `config`, `objects/`, etc. |
| Worktree | Text file | `.git` file containing `gitdir: /path/to/repo/.git/worktrees/name` |

**Detection code:**
```go
// internal/git/worktree.go
func IsWorktree(path string) bool {
    gitPath := filepath.Join(path, ".git")
    info, err := os.Stat(gitPath)
    if err != nil {
        return false
    }
    return !info.IsDir() // File = worktree, Directory = main repo
}
```

**Finding main repo from worktree:**
```go
func GetMainRepoPath(worktreePath string) (string, error) {
    // 1. Read .git file
    content, _ := os.ReadFile(filepath.Join(worktreePath, ".git"))

    // 2. Parse "gitdir: /path/to/repo/.git/worktrees/name"
    gitdir := strings.TrimPrefix(string(content), "gitdir: ")

    // 3. Walk up: .git/worktrees/name -> .git/worktrees -> .git -> repo
    // Return parent of .git directory
}
```

---

## Worktree Lifecycle

### Creating Worktrees

**Two modes:**

1. **New branch** - creates branch and worktree together:
   ```bash
   git worktree add /path/to/worktree -b new-branch base-ref
   ```

2. **Existing branch** - checks out existing branch:
   ```bash
   git worktree add /path/to/worktree existing-branch
   ```

**Checks before creation:**
- Branch doesn't already exist (for new branch mode)
- Branch exists locally (for existing branch mode)
- Branch not already checked out in another worktree

**Error: "already checked out":**
```
fatal: 'feature-x' is already checked out at '/other/path'
```
The tool handles this by returning the existing path with `AlreadyExists=true`.

### Listing Worktrees

**From main repo:**
```bash
git worktree list --porcelain
```

Output format:
```
worktree /path/to/main-repo
HEAD abc1234...
branch refs/heads/main

worktree /path/to/feature-x
HEAD def5678...
branch refs/heads/feature-x
```

**Detached HEAD:** Shows `detached` instead of `branch refs/heads/...`

### Moving Worktrees

```bash
git worktree move /old/path /new/path
```

**Critical consideration:** If moving a repo that has worktrees, you must:
1. Move nested worktrees OUT of the repo first
2. Move the repo
3. Run `git worktree repair` to fix links

### Removing Worktrees

```bash
git worktree remove /path/to/worktree        # Safe remove
git worktree remove --force /path/to/worktree # Force remove (dirty/unmerged)
```

**Cleanup stale references:**
```bash
git worktree prune
```

---

## Branch and Merge Status

### Current Branch

```bash
git branch --show-current
```
Returns empty string if in detached HEAD state.

### Default Branch Detection

Priority order:
1. `git symbolic-ref refs/remotes/origin/HEAD` (most reliable)
2. Check if `origin/main` exists
3. Check if `origin/master` exists
4. Fallback to "main"

### Merge Status

**Is branch merged into default?**
```bash
git branch --merged origin/main
```

Output markers:
- `  feature-x` - plain branch name
- `* feature-x` - current branch
- `+ feature-x` - checked out in a worktree

### Commits Ahead/Behind

```bash
# Commits ahead of default
git rev-list --count origin/main..feature-x

# Commits behind default
git rev-list --count feature-x..origin/main
```

### Dirty Status

```bash
git status --porcelain
```
Non-empty output = dirty (uncommitted changes).

### Upstream Tracking

```bash
git config branch.feature-x.merge
# Returns: refs/heads/feature-x (or empty if no upstream)
```

---

## The .git File and gitdir Links

### Bidirectional Link Structure

```
worktree/.git (file)
    └── gitdir: /repo/.git/worktrees/worktree-name

/repo/.git/worktrees/worktree-name/gitdir (file)
    └── /path/to/worktree/.git
```

### Link Validation

A valid worktree link requires:
1. `.git` file exists and is readable
2. `gitdir:` points to existing directory
3. That directory contains `gitdir` file pointing back

### Repairing Broken Links

```bash
# Repair all worktrees for a repo
git worktree repair

# Repair specific worktree
git worktree repair /path/to/worktree
```

### Finding Prunable Worktrees

```bash
git worktree prune --dry-run -v
```
Output: `Removing /path: reason`

---

## Repository Discovery

### Identifying Main Repos

A directory is a main repo if:
- Contains `.git` **directory** (not file)
- Has valid git structure

### Finding Repos by Name

```go
// Scan direct children of basePath
entries, _ := os.ReadDir(basePath)
for _, entry := range entries {
    if isMainRepo(filepath.Join(basePath, entry.Name())) {
        // Found a repo
    }
}
```

### Current Repo Path

```bash
git rev-parse --show-toplevel
```
Returns the root of the current repo (works from subdirectories).

---

## Metadata Storage

### Branch Notes (Descriptions)

Git's built-in branch description feature:

```bash
# Get
git config branch.feature-x.description

# Set
git config branch.feature-x.description "Working on login feature"

# Clear
git config --unset branch.feature-x.description
```

### Repository Labels

Custom config stored in repo's local config:

```bash
# Stored as comma-separated values
git config --local wt.labels "backend,critical"

# Get
git config --local wt.labels

# Set (replaces all)
git config --local wt.labels "frontend,ui"

# Clear
git config --local --unset wt.labels
```

### Batch Config Reading

```bash
git config --get-regexp 'branch\.'
```

Output:
```
branch.main.remote origin
branch.main.merge refs/heads/main
branch.feature-x.description My feature note
branch.feature-x.merge refs/heads/feature-x
```

---

## Performance Patterns

### Batching Git Calls

Instead of calling git once per worktree, batch by repo:

```go
// Bad: O(n) calls
for _, wt := range worktrees {
    branch := getBranch(wt.Path)      // 1 call
    merged := isMerged(wt.MainRepo, branch) // 1 call
    // ... more calls
}

// Good: O(repos) calls
for repo, worktrees := range groupByRepo(allWorktrees) {
    mergedBranches := getMergedBranches(repo) // 1 call for all
    configs := getAllBranchConfig(repo)       // 1 call for all
    // Apply to all worktrees in repo
}
```

### Batched Operations Used

| Operation | Command | Returns |
|-----------|---------|---------|
| All worktrees | `git worktree list --porcelain` | All worktrees for repo |
| All merged branches | `git branch --merged origin/main` | Set of merged branches |
| All branch configs | `git config --get-regexp 'branch\.'` | Notes + upstreams |

### Typical Call Count

For 10 worktrees across 2 repos:
- Without dirty check: ~8 git calls
- With dirty check: ~18 git calls (adds 1 per worktree)

---

## Common Git Commands Reference

| Purpose | Command |
|---------|---------|
| Create worktree (new branch) | `git worktree add <path> -b <branch> <base>` |
| Create worktree (existing branch) | `git worktree add <path> <branch>` |
| List worktrees | `git worktree list --porcelain` |
| Move worktree | `git worktree move <old> <new>` |
| Remove worktree | `git worktree remove [--force] <path>` |
| Repair links | `git worktree repair [<path>]` |
| Prune stale | `git worktree prune` |
| Current branch | `git branch --show-current` |
| Merged branches | `git branch --merged <ref>` |
| Commits ahead | `git rev-list --count <base>..<branch>` |
| Origin URL | `git remote get-url origin` |
| Repo root | `git rev-parse --show-toplevel` |
| Dirty check | `git status --porcelain` |
| Fetch branch | `git fetch origin <branch> --quiet` |

---

## Error Handling Philosophy

**Read operations:** Return safe defaults on error
- Missing branch → empty string
- Merge check fails → assume not merged
- Dirty check fails → assume clean

**Write operations:** Fail explicitly
- Create, move, remove → return error to caller
- User must handle or see the error

---

## See Also

- [Git Worktree Documentation](https://git-scm.com/docs/git-worktree)
- [internal/git/](../internal/git/) - Implementation details
- [CLAUDE.md](../CLAUDE.md) - Development guidelines
