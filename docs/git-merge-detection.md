# Detecting Merged Git Branches Programmatically

## Two-Stage Detection Approach

### 1. Ancestry-based detection (regular merges)

```bash
git merge-base --is-ancestor <branch> <target-branch>
```

- Returns exit code 0 if branch is an ancestor of target
- Works for regular merges and fast-forward merges
- **Fails for squash merges** (commits aren't ancestors)

### 2. Patch-ID detection (squash merges)

```bash
git cherry <target-branch> <branch>
```

- Compares patch content, not commit hashes
- Lines prefixed with `+` = commits unique to branch (unmerged)
- Lines prefixed with `-` = equivalent patches exist in target (merged)
- Only works for single-commit branches or identical patch content

## GitHub CLI Enhancement

For accurate squash-merge detection with multi-commit branches:

```bash
gh pr list --state merged --json headRefName --limit 200
```

- Returns branch names of merged PRs regardless of merge strategy
- GitHub tracks the actual merge event, not commit reachability
- Works because GitHub knows "this PR was merged" even if commits are squashed/rebased

## Why Git-Only Detection Fails for Multi-Commit Squash Merges

```
# Regular merge - commits A,B,C are reachable from main
feature: A -> B -> C
                    \
main:    X -> Y -> Z -> M (merge commit)

# Squash merge - S is a NEW commit, A,B,C are orphaned
feature: A -> B -> C  (not reachable from main!)
main:    X -> Y -> Z -> S (squashed commit)
```

- Squash merge creates NEW commit S from commits A,B,C
- Commit S has different hash than A,B,C
- `git merge-base --is-ancestor` fails: A,B,C aren't ancestors of main
- `git cherry` fails: patch-id of S ≠ combined patch-ids of A,B,C
- Only GitHub API knows that S represents the merge of that PR

## Shell Integration for Directory-Changing CLIs

Child processes cannot modify parent shell's working directory. Pattern:

```bash
my_tool() {
  if [[ "$1" == "switch" ]]; then
    local target_dir
    target_dir=$(command my_tool switch "${@:2}")
    cd "$target_dir"
  else
    command my_tool "$@"
  fi
}
```

- `command` builtin bypasses function to call actual binary
- Same pattern used by: nvm, pyenv, direnv, zoxide
- This is why `cd` is a shell builtin, not an external binary
