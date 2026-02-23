---
name: codecov
description: >
  Fetches test coverage data from Codecov API and identifies coverage gaps.
  Use when asking about test coverage, finding untested code, or planning
  coverage improvements. Supports optional path filtering.
allowed-tools: WebFetch Bash(git remote:*)
argument-hint: "[path-filter]"
---

# Codecov Coverage Analysis

Fetch and analyze test coverage data from Codecov for this repository.

## Steps

1. **Derive owner/repo** from git remote:
   ```bash
   git remote get-url origin
   ```
   Parse `github.com/{owner}/{repo}` (strip `.git` suffix if present).

2. **Fetch overall report** from:
   ```
   https://codecov.io/api/v2/github/{owner}/repos/{repo}/report/?page_size=50
   ```

3. **Apply path filter** if `$ARGUMENTS` is provided:
   - Use `?path={ARGUMENTS}&page_size=50` to filter by directory (e.g., `internal`, `cmd/wt`)
   - Otherwise fetch both `?path=cmd&page_size=50` and `?path=internal&page_size=50`

4. **Extract per-file coverage** from the response:
   - Overall totals are in `response.totals` (`coverage`, `lines`, `hits`, `misses`, `partials`)
   - Per-file data is in `response.files[]` with fields: `name`, `totals.coverage`, `totals.lines`, `totals.hits`, `totals.misses`

5. **Sort files by missed lines descending** and present:
   - Overall coverage percentage
   - Files with 0% coverage (completely untested)
   - Top files by missed lines (partially tested)
   - Actionable recommendations for improving coverage

## Output Format

```
## Coverage Report: {owner}/{repo}

Overall: {coverage}% ({hits}/{lines} lines covered)

### Untested Files (0% coverage)
| File | Lines |
|------|-------|
| path/to/file.go | 42 |

### Top Coverage Gaps (by missed lines)
| File | Coverage | Missed | Total |
|------|----------|--------|-------|
| path/to/file.go | 45.2% | 74 | 135 |

### Recommendations
- ...
```
