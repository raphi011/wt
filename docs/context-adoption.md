# Adopting Go's `context.Context`

This document outlines the use cases for introducing Go's `context` package into the wt codebase.

## Implementation Status

### Completed (Phase 1)

- [x] `internal/log` package with context-aware logger
- [x] `internal/cmd/exec.go` - `RunContext` and `OutputContext` with verbose logging
- [x] `--verbose` / `-v` global flag
- [x] Signal handling in `main.go` (`signal.NotifyContext`)

### Completed (Phase 2)

- [x] `internal/git/exec.go` - `runGit` and `outputGit` context-only helpers
- [x] `internal/git/repo.go` - all functions migrated to context
- [x] `internal/git/worktree.go` - all functions migrated to context
- [x] `internal/git/notes.go` - all functions migrated to context
- [x] `internal/git/labels.go` - all functions migrated to context
- [x] `internal/git/check.go` - all functions migrated to context
- [x] `internal/forge/forge.go` - interface updated with context parameters
- [x] `internal/forge/github.go` - all methods migrated to context
- [x] `internal/forge/gitlab.go` - all methods migrated to context
- [x] Removed non-context versions of exec functions

### Remaining (Phase 3)

- [ ] Update command implementations in `cmd/wt/` to pass context to git/forge operations
- [ ] Add timeouts for network operations
- [ ] Add timeouts for hook execution
- [ ] Remove explicit `io.Writer` parameters where logger suffices

## Use Cases

## Use Cases

### 1. Cancellation (SIGINT/SIGTERM)

**Problem**: Long-running operations (git fetch, PR API calls, hook execution) cannot be interrupted gracefully.

**Solution**: Use `signal.NotifyContext` in main and pass context through the call stack.

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()
```

All `exec.Command` calls become `exec.CommandContext(ctx, ...)`, allowing in-flight operations to be cancelled when the user presses Ctrl+C.

**Affected code**:
- `internal/cmd/exec.go` - command execution wrapper
- `internal/git/` - all git operations
- `internal/forge/` - GitHub/GitLab API calls
- `cmd/wt/prune.go` - concurrent PR fetching

### 2. Timeouts

**Problem**: Network operations can hang indefinitely (git fetch, clone, API calls). User-defined hooks have no timeout protection.

**Solution**: Wrap specific operations with `context.WithTimeout`.

```go
// Hook execution with 30s timeout
hookCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

// Git fetch with 2m timeout
fetchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
defer cancel()
```

**Affected code**:
- `internal/hooks/hooks.go` - hook execution
- `internal/git/repo.go` - `FetchBranch()`
- `internal/forge/github.go` - `CloneRepo()`
- `internal/forge/gitlab.go` - `CloneRepo()`

### 3. Logging and Verbose Mode

**Problem**: No way to see what external commands are being executed. Output writer (`Stdout`) must be passed explicitly through multiple function layers.

**Solution**: Store a logger in context that can be retrieved anywhere in the call stack.

```go
// internal/log/log.go
type Logger struct {
    out     io.Writer
    verbose bool
}

func WithLogger(ctx context.Context, l *Logger) context.Context
func FromContext(ctx context.Context) *Logger

func (l *Logger) Printf(format string, args ...any)
func (l *Logger) Command(name string, args ...string)  // logs in verbose mode
func (l *Logger) Writer() io.Writer
```

**Usage**:
```go
// In main.go
logger := log.New(os.Stdout, verbose)
ctx = log.WithLogger(ctx, logger)

// Anywhere in codebase
log.FromContext(ctx).Printf("Processing %s\n", branch)

// In exec wrapper - automatically logs commands when verbose
func OutputContext(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
    log.FromContext(ctx).Command(name, args...)  // prints: $ git status
    cmd := exec.CommandContext(ctx, name, args...)
    // ...
}
```

**Benefits**:
- `wt list -v` shows all git/gh/glab commands being executed
- No need to pass `io.Writer` through every function
- Consistent logging pattern across codebase
- Easier testing (inject buffer via context)

### 4. Request-Scoped Values

**Problem**: Flags like `--dry-run` need to be threaded through many function calls.

**Solution**: Store in context for deep access.

```go
ctx = context.WithValue(ctx, dryRunKey, true)

// Deep in call stack
if isDryRun(ctx) {
    log.FromContext(ctx).Printf("would run: %s\n", cmd)
    return nil
}
```

**Note**: Use sparingly. Explicit parameters are often clearer. Best for cross-cutting concerns that would otherwise require changing many function signatures.

### 5. Testing

**Problem**: Tests need to inject fake clocks, control randomness, or mock dependencies.

**Solution**: Store test doubles in context.

```go
// Production
ctx = withClock(ctx, realClock{})

// Test
ctx = withClock(ctx, fakeClock{now: fixedTime})
```

### 6. Tracing and Debugging

**Problem**: When errors occur deep in the stack, it's hard to know which operation failed.

**Solution**: Attach operation metadata to context.

```go
ctx = withOperation(ctx, "prune")
ctx = withWorktree(ctx, wt.Branch)

// In error handling
return fmt.Errorf("%s: git %v failed: %w", getOperation(ctx), args, err)
```

## Implementation Priority

| Use Case | Priority | Effort | Value |
|----------|----------|--------|-------|
| Logging/verbose mode | High | Medium | High visibility into tool behavior |
| Cancellation (SIGINT) | High | Medium | Prevents hung processes |
| Timeouts | Medium | Low | Reliability for network ops |
| Dry-run propagation | Low | Low | Cleaner than threading bool |
| Request-scoped values | Low | Low | Case-by-case basis |
| Tracing | Low | Low | Debugging aid |

## Implementation Plan

### Phase 1: Core Infrastructure

1. Create `internal/log` package with context-aware logger
2. Update `internal/cmd/exec.go` to use `exec.CommandContext` and log commands
3. Add `--verbose` / `-v` global flag
4. Wire up `signal.NotifyContext` in `main.go`

### Phase 2: Propagate Context

1. Add `context.Context` parameter to `internal/git` functions
2. Add `context.Context` parameter to `internal/forge` interface and implementations
3. Update command implementations to pass context through

### Phase 3: Timeouts and Polish

1. Add configurable timeouts for network operations
2. Add timeout for hook execution
3. Remove explicit `io.Writer` parameters where logger suffices

## API Changes

The `Forge` interface will need context parameters:

```go
// Before
type Forge interface {
    GetPRForBranch(repoURL, branch string) (*PR, error)
    CloneRepo(repo, targetDir string) error
    // ...
}

// After
type Forge interface {
    GetPRForBranch(ctx context.Context, repoURL, branch string) (*PR, error)
    CloneRepo(ctx context.Context, repo, targetDir string) error
    // ...
}
```

Similarly for `internal/git` functions.
