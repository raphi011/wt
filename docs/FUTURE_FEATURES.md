# Future Feature Recommendations

This document outlines potential features to improve `wt` based on common patterns in worktree management tools.

## High Priority

### File Preservation on Worktree Creation

**Problem:** When creating a new worktree, users must manually copy configuration files like `.env`, `.envrc`, or `docker-compose.override.yml`. These files are typically gitignored and contain local development settings.

**Solution:** Automatically copy matching files from an existing worktree (or main repo) when creating a new worktree.

**Config:**
```toml
[preserve]
patterns = [".env*", ".envrc", "docker-compose.override.yml"]
exclude = ["node_modules", "vendor", ".venv", "dist", "build", "__pycache__"]
```

**Behavior:**
- On `wt checkout`, find gitignored files matching `patterns`
- Skip files in directories matching `exclude`
- Copy to new worktree (never overwrite existing)
- Use atomic file operations to avoid race conditions

**CLI:**
```bash
wt checkout -b feat/auth                    # Auto-preserve from current worktree
wt checkout -b feat/auth --from main        # Preserve from specific worktree
wt checkout -b feat/auth --no-preserve      # Skip file preservation
```

---

## Medium Priority

### Workspace Locking

**Problem:** Concurrent `wt` commands operating on the same worktree directory could race and corrupt state.

**Solution:** Implement workspace-level locking for mutating operations.

**Implementation:**
- Create `.wt.lock` file with PID on mutating operations (`checkout`, `prune`, `mv`)
- Check if lock holder process is still running (stale lock detection)
- Add max age for locks (e.g., 30 minutes) to mitigate PID reuse
- Retry with backoff on lock contention

**Affected commands:** `checkout`, `prune`, `mv`

---

### Age-Based Pruning

**Problem:** `wt prune` only considers PR merge status. Old worktrees with no upstream or stale experiments accumulate.

**Solution:** Add time-based pruning options.

**CLI:**
```bash
wt prune --older-than 30d          # Prune worktrees with no commits in 30 days
wt prune --older-than 2w           # 2 weeks
wt prune --stale                   # Use configured threshold (default: 30d)
```

**Config:**
```toml
stale_threshold = "30d"  # Default for --stale flag
```

**Behavior:**
- Check last commit time in worktree
- `--older-than` cannot be combined with PR-based pruning (different use cases)
- Still respects dirty status (won't prune dirty worktrees without `--force`)

---

### Enhanced Move Command

**Problem:** `wt mv` moves the worktree directory but doesn't rename the git branch. Users expect both to change together.

**Solution:** Atomic branch + directory rename with rollback on failure.

**CLI:**
```bash
wt mv feat/old feat/new            # Rename branch AND directory
wt mv --branch feat/old feat/new   # Target by branch name
```

**Implementation:**
1. Validate: worktree not dirty, not locked, new branch doesn't exist
2. Rename git branch (`git branch -m old new`)
3. Calculate new directory name from branch
4. Move worktree (`git worktree move`)
5. Repair worktree links if needed
6. On any failure: rollback all changes

**Constraints:**
- Cannot rename current worktree (must switch first)
- Cannot rename if new branch name already exists

---

## Low Priority

### Doctor Command

**Problem:** Worktree links can become broken (moved repos, deleted directories). Users need a way to diagnose and repair.

**Solution:** Add `wt doctor` command for diagnostics and repair.

**CLI:**
```bash
wt doctor              # Check all worktrees for issues
wt doctor --repair     # Attempt to fix found issues
```

**Checks:**
- Broken bidirectional links (worktree ↔ main repo)
- Missing upstream branches
- Stale cache entries
- Orphaned worktree references in git

**Repairs:**
- `git worktree repair` for broken links
- Cache cleanup for stale entries
- `git worktree prune` for orphaned refs

---

### Lock/Unlock Commands

**Problem:** Users may want to protect important worktrees from accidental deletion via `wt prune`.

**Solution:** Add worktree locking mechanism.

**CLI:**
```bash
wt lock --branch feat/important    # Lock worktree by branch
wt lock -r myrepo                  # Lock repo's current worktree
wt unlock --branch feat/important  # Remove lock
wt list                            # Shows 🔒 indicator for locked
```

**Config:**
```toml
auto_lock_patterns = ["main", "master", "develop", "release/*"]
```

**Behavior:**
- Locked worktrees skip during `wt prune`
- `wt prune --force` still requires explicit confirmation for locked worktrees
- Auto-lock matching branches on creation

---

## Implementation Notes

### Priority Order

1. **File preservation** - High user value, reduces friction
2. **Workspace locking** - Safety improvement, relatively simple
3. **Age-based pruning** - Common request, complements existing prune
4. **Enhanced move** - Fixes incomplete UX in existing command
5. **Doctor/Lock** - Nice-to-have, lower urgency

### Breaking Changes

None of these features require breaking changes. All can be implemented as additive features with sensible defaults (disabled or opt-in).

### Config Schema Additions

```toml
# File preservation
[preserve]
patterns = [".env*", ".envrc"]
exclude = ["node_modules", "vendor"]

# Pruning
stale_threshold = "30d"

# Locking
auto_lock_patterns = ["main", "master"]
```
