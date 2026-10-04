# Dependency refresh and performance/UI improvement plan

Date: 2026-10-03

## Scope and sequencing

Refresh dependencies and assess their APIs before implementing the eight review findings. Keep the dependency baseline separate from behavior changes so regressions are attributable. This document records the upgrade, adoption decisions, implementation order, and acceptance criteria.

The initial review's `go test ./...` passed. Executable probes reproduced an existing `main (worktree)` option being selected as a new branch, Left navigating back while filtering, and a 33-line wizard in an 80×24 terminal. Performance opportunities were identified from subprocess call paths; latency improvements have not yet been benchmarked.

## Phase 0 — Dependency baseline

- [x] Resolve latest versions using Go module metadata and review relevant upstream releases.
- [x] Update dependencies with `go get -u ./...` and `go get -u -t ./...`.
- [x] Update the explicitly required indirect golden module to its latest tagged release.
- [x] Run `go mod tidy`.
- [x] Complete build, race-enabled tests, vet, and module verification; record results below.
- [x] Assess API adoption opportunities before implementing review fixes.

### Direct dependencies

Latest versions for the existing module paths, checked on the date above:

| Module | Previous | Selected |
| --- | --- | --- |
| `charm.land/bubbles/v2` | v2.1.0 | v2.2.1 |
| `charm.land/bubbletea/v2` | v2.0.4 | v2.0.10 |
| `charm.land/lipgloss/v2` | v2.0.3 | v2.0.6 |
| `github.com/BurntSushi/toml` | v1.6.0 | unchanged |
| `github.com/atotto/clipboard` | v0.1.4 | unchanged |
| `github.com/charmbracelet/colorprofile` | v0.4.3 | unchanged |
| `github.com/charmbracelet/x/ansi` | v0.11.7 | v0.11.8 |
| `github.com/mattn/go-isatty` | v0.0.21 | v0.0.24 |
| `github.com/sahilm/fuzzy` | v0.1.1 | v0.1.3 |
| `github.com/spf13/cobra` | v1.10.2 | unchanged |
| `golang.org/x/sync` | v0.20.0 | v0.23.0 |

Indirect upgrades: Ultraviolet to `v0.0.0-20261001125412-878653296cfd`, golden to v0.1.0, go-colorful to v1.4.1, go-runewidth to v0.0.30, terminfo to v1.2.0, and x/sys to v0.48.0. Ultraviolet remains a pseudo-version because its module is untagged. Go normalized the directive from `1.26` to `1.26.0`; the local toolchain is Go 1.26.0.

Remaining updates reported by `go list -m -u all` are outside this module's package/test dependency closure: bitset, go-md2man/v2, go-humanize, yaml/v3, x/exp, x/mod, x/tools, and check.v1. `go mod why -m` reports that this module does not need them. Do not add artificial requirements solely to update unused portions of upstream module graphs.

This pass covers Go modules. GitHub Actions, GoReleaser, and installed Git/gh/glab tools are separate tooling upgrades.

### Adoption assessment

The repository already uses Charm v2. Most useful changes are better use of existing v2 APIs, rather than a second migration or wholesale UI rewrite.

| Capability | Decision | Application |
| --- | --- | --- |
| Bubble Tea v2.0.10 skips capability queries when input is disabled | Adopt `tea.WithInput(nil)` for the display-only progress program in the UI work | Prevent progress from consuming keyboard input; retain command-context signal cancellation and add plain stderr behavior for non-TTY output. |
| Bubble Tea command/message model and `tea.WithContext` | Adopt in phase 2; these APIs predate this upgrade | Move fetches out of `Update`, carry cancellation into subprocesses, and deliver typed results back to the model. |
| Bubbles list/viewport sizing, textinput key maps, key/help components | Reuse selectively in phases 1–3; these are existing v2 capabilities | Generate help from active bindings and share dimension-aware rendering. Evaluate `list.Model` for PR selection; preserve custom multi-select, disabled options, and create-from-filter behavior. |
| Textinput real cursor (`SetVirtualCursor(false)`, `Cursor()`) | Defer until layout offsets are explicit | A real cursor must be positioned relative to title, tabs, border, and padding. First forward non-key messages so virtual cursor blink commands work. |
| Bubble Tea background-color request/messages | Evaluate during phase 2 | Current global theme initialization performs synchronous terminal background detection. Prefer in-program detection for interactive flows, explicit theme overrides, and a non-blocking fallback for completion/plain output. |
| Lip Gloss v2.0.6 table shrinking and Unicode rendering fixes | Receive fixes through upgrade; use `Table.Width`, `Wrap`, and display-width measurement in phase 3 | Terminal-aware layout still requires application code. Keep labels, notes, emoji, and combining characters aligned. |
| Fuzzy v0.1.3 iterator sources (`FindFromIter`) | Keep the current `Source` adapter | Existing options are already a slice and the adapter avoids building a separate label slice. An iterator rewrite has no demonstrated allocation or latency benefit. Preserve ranking and match-index semantics. |
| Bubbles v2.2 tree component and textarea selection | Defer | Current flows use flat selectors and single-line input. These features do not solve the reviewed issues. |
| x/sync errgroup limits | Reuse the existing bounded-loader approach in phase 5 | Preserve partial results and deterministic ordering; do not turn one failed repository into failure of all completions. |

Sources: [Bubble Tea releases](https://github.com/charmbracelet/bubbletea/releases), [Bubbles releases](https://github.com/charmbracelet/bubbles/releases), [Lip Gloss v2.0.6](https://github.com/charmbracelet/lipgloss/releases/tag/v2.0.6), [fuzzy releases](https://github.com/sahilm/fuzzy/releases). API choices were also checked against downloaded source for the selected versions.

## Phase 1 — Correct branch selection and keyboard behavior

Review findings: **1, 4, 8**.

Files: `internal/ui/wizard/steps/filterable_list.go`, `text_input.go`, `framework/wizard.go`, `framework/step.go`, shared bindings in `framework/keys.go`, `flows/checkout.go`, and the lightweight `flows/cd.go` wrapper.

- [x] Separate canonical option identity/search text from decorated labels. Compare branch creation against the underlying branch name; keep `(worktree)` as presentation metadata. Respect Git's case-sensitive branch identity rather than imposing case-insensitive uniqueness.
- [x] Let focused inputs handle Left/Right and word-navigation keys. Use explicit step navigation such as Alt+Left/Alt+Right; preserve arrows for navigation when list focus makes that unambiguous.
- [x] Make Ctrl+C cancel immediately. Retain Esc as clear-filter-then-cancel only with accurate contextual help; confirmation prompts can keep immediate cancellation.
- [x] Centralize bindings/help and forward component messages, including cursor blink messages, through both the wizard and step layers. Run step initialization appropriately when navigating back.

Acceptance:

- Filtering `main` with an existing `main (worktree)` entry never offers to create `main`; selecting it opens the existing branch.
- A genuinely new branch remains creatable, including distinct case-sensitive names where Git permits them.
- Editing in the middle of a filter preserves the caret and does not confirm or change steps.
- Ctrl+C cancels with empty or nonempty input; Esc behavior matches visible help.
- Multi-select Space, paste sanitization, disabled entries, summaries, and back navigation still work.

### Phase 1 implementation results

Implemented on 2026-10-03:

- Options now carry optional canonical `SearchText`, falling back to the label for existing callers. Checkout supplies the branch name separately from its `(worktree)` label. Creation checks are case-sensitive; fuzzy ranking remains unchanged and presentation metadata is excluded from branch search. Highlighting uses canonical match positions only when they align with the label prefix.
- Focused filters and text fields retain Left/Right for caret movement. Alt+Left/Alt+Right navigate steps; Ctrl+Left/Ctrl+Right and Alt+B/Alt+F remain word-navigation keys. Unfocused lists retain plain arrow navigation. Shared navigation/cancellation bindings generate contextual help, including Escape's current clear/cancel action and immediate Ctrl+C cancellation.
- Wizard and input steps forward component messages, restart initialization on back navigation (including from summary), and preserve transition commands. The lightweight cd wrapper now forwards cursor messages and uses the same cancellation bindings.
- Added regressions for decorated branch selection and summaries, case-sensitive creation, canonical search, editing at the caret, word movement, cancellation with populated input, focus-specific multi-select help, and blink initialization through both wizard layers and cd. Existing paste sanitization, disabled-entry, multi-select, skip-condition, and summary tests still pass.

Validation: `go test -race ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` passed. Automated tests drive Bubble Tea messages; manual terminal/color/size checks have not been run and remain part of the UI delivery validation below. Phase 2 is next; synchronous branch/PR fetching and fixed layout sizing remain pending.

## Phase 2 — Keep wizard I/O asynchronous

Review finding: **2**, plus the progress and theme adoption opportunities above.

Files: `framework/wizard.go`, `framework/step.go`, `flows/checkout.go`, `flows/pr_checkout.go`, `cmd/wt/checkout_cmd.go`, `cmd/wt/pr_cmd.go`, `steps/loading.go`, `internal/ui/progress/progress_bar.go`, `internal/ui/styles/theme.go`, and `internal/config/resolver.go`.

- [x] Allow completion handlers to produce commands, or introduce explicit asynchronous loading steps. Fetch commands return typed messages; only `Update` mutates UI state.
- [x] Add loading, empty, error, and retry states for branch/PR selection.
- [x] Pass context through wizard execution and fetchers. Cancel obsolete requests on repository change and on exit; ignore late results using request IDs.
- [x] Reuse results when returning to an unchanged repository, with an explicit refresh path.
- [x] Disable input for the progress-only program and skip interactive rendering/line-clearing escapes on non-TTY stderr.
- [x] Evaluate moving interactive auto-theme detection into Bubble Tea's event loop without delaying shell completion or changing explicit light/dark configuration.

Acceptance:

- A controlled slow fetch leaves resize and cancellation responsive.
- Switching from repository A to B cannot show A's late response in B's step.
- Errors and zero results have actionable states; GitHub and GitLab retain parity.
- Progress does not consume input intended for another command and exits cleanly on cancellation.
- Stdout stays suitable for paths and JSON; redirected stderr contains readable diagnostics.

### Phase 2 implementation results

Implemented on 2026-10-03:

- Added a reusable loading wrapper around existing selectors. Its commands perform context-aware I/O and return typed results; the event loop applies options, branch/base defaults, and errors. Back navigation, exit, and refresh cancel requests; monotonically increasing request IDs reject obsolete results, including late replies for the same repository.
- Branch and PR wizard construction no longer fetches data. Loading blocks confirmation while preserving resize/back/cancellation. Errors and empty results expose Ctrl+R retry/refresh. Successful results are cached per repository; returning to an unchanged repository preserves selection, and changing repository resets dependent branch/base data. An empty branch list allows creating the first branch without requiring a nonexistent base.
- Fetchers now take context, and branch fetchers return errors instead of silently converting failures into empty lists. Wizard execution uses the caller's context and treats external context cancellation as a cancelled result. The shared configuration resolver now serializes cache access so overlapping cancelled/replacement fetches can safely resolve per-repo settings.
- Connected the previously unused PR wizard to `wt pr checkout -i [repo]`, retaining GitHub/GitLab detection, configured accounts, explicit hook flags, and the existing checkout path after selection. Interactive mode supports registered local repositories and existing remote matches; cloning remains the explicit noninteractive path.
- The progress program uses `tea.WithInput(nil)` and the command context. Non-TTY stderr gets readable newline-delimited diagnostics with counts; no Bubble Tea program or line-clearing escapes are emitted. Repeated Stop calls and updates after cancellation are safe.
- Removed synchronous terminal background queries from theme initialization. Auto mode starts with a dark fallback; interactive wizard/cd programs request the background through Bubble Tea and apply the reply while preserving custom colors. Explicit light/dark and colorless themes do not query. Plain output and shell completion do not wait for a terminal reply.

Validation:

- `go test -race ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` passed.
- Regression coverage includes slow-fetch resize/cancellation, repository switches, stale same-repository responses, cache isolation, retry/refresh, empty/error confirmation blocking, branch defaults/first-branch creation, parent cancellation, concurrent config resolution, and theme overrides.
- Fake `gh` and `glab` executables in isolated local Git fixtures verified equivalent PR fields and cancellation of running forge subprocesses; no live forge operations were used.
- PTY smoke probes exercised checkout, PR checkout, cd, and prune at 80×24, 120×40, and 40×12 with light/dark/colorless themes (36 combinations), including resize, slow-load cancellation, cd caret editing/selection, and JSON stdout isolation. A separate PTY progress probe confirmed stdin remains available to the caller. Redirected progress diagnostics and cancellation were checked in automated tests.

Small-terminal content still clips with the existing layout; the PTY checks validate interaction and cancellation, not layout fit. Phase 3 is next. Human visual inspection, live forge compatibility, and the final shell-completion/output delivery checks remain pending.

## Phase 3 — Responsive shared list and table layouts

Review finding: **3**.

Files: `framework/wizard.go`, `framework/step.go`, `steps/filterable_list.go`, `steps/single_select.go`, `flows/pr_checkout.go`, and `internal/ui/static/table.go`.

- [x] Propagate terminal dimensions to the active step. Calculate content space after title, tabs, information, help, border, and padding.
- [x] Replace the ten-item limit with a row budget that accounts for descriptions and wrapping. Keep the focused option visible after resize/filtering.
- [x] Share scrolling behavior across single-select and multi-select; use a viewport/list component where it reduces custom logic without losing selection semantics.
- [x] Constrain input width, tabs, summaries, labels, and descriptions. Provide a compact fallback for very small terminals.
- [x] Supply terminal width to static table rendering. Prioritize repo/branch/status; wrap or truncate notes deliberately and preserve full JSON output.

Acceptance:

- Render at 80×24, 120×40, and 40×12, and resize while a filter/selection is active.
- Navigation, focused rows, and help remain visible; long lists do not render every option.
- Check empty/all-disabled lists, long branch names/notes, CJK text, combining marks, and emoji.
- Piped table output has a deliberate stable format independent of an unavailable terminal width.

### Phase 3 implementation results

Implemented on 2026-10-03:

- Added optional content-area sizing for steps. The wizard budgets rows and display cells after its title, information, tabs, contextual help, border, margins, and padding; resizing and step transitions update the active step. Narrow/short terminals use compact chrome and contextual bindings. Overflowing tabs fall back to the active step title.
- Single-select and filterable single/multi-select share a row-aware scrolling renderer. It measures wrapped descriptions, renders only the visible options, and always includes the focused label. Labels truncate by display width; descriptions wrap to at most three rows with an ellipsis. Small row budgets prioritize the focused option, and larger budgets retain scroll indicators. Existing selection, disabled-option, create-from-filter, and fuzzy-match semantics remain intact; a Bubbles list/viewport replacement was unnecessary for this shared renderer.
- Input widths follow the available content area. The lightweight cd flow now budgets its own contextual help and responds to terminal resize. Loading wrappers pass dimensions to their selectors, including the space needed for empty-state messages. Text input retains its editable row in compact layouts. Summaries wrap and scroll with Up/Down, with bounded scrolling and preserved back navigation.
- Table commands now measure their actual output destination. Terminal tables constrain columns to that width, deliberately truncate long cells/notes, and hide note/age/commit columns below 60 cells to prioritize repository, branch, and PR. Notes have a 24-cell maximum in terminal tables. Pipes/files retain the existing full-width, full-column format; JSON and source rows remain unchanged. The existing x/term dependency is now direct for destination sizing.

Validation:

- `go test -race ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` passed.
- Added regressions at 80×24, 120×40, and 40×12 for wizard/cd layouts and loaded PRs, including focused-row and help visibility, resize with filters/selection, long lists/descriptions, multi-select, empty/all-disabled options, create identity, CJK, combining marks, emoji, and summary scrolling/back navigation. Tests also verify width-constrained tables, full notes in redirected output, unchanged source rows, and destination-aware sizing.
- Re-ran the 36-case PTY smoke matrix for checkout, PR checkout, cd, and prune across the three terminal sizes and light/dark/colorless themes, including resize, slow-load cancellation, cd caret editing/selection, and JSON stdout isolation. Inspected generated plain-text layout samples for long Unicode labels/descriptions at all three sizes. PTY smoke probes use local fixtures and do not perform live forge operations.

Phase 4 is next. Human visual inspection in a terminal, live forge compatibility, and final shell-completion/output delivery checks remain pending; generated view assertions and PTY interaction checks cover automated behavior and layout bounds.

## Phase 4 — PR status and output consistency

Review finding: **7**.

Files: `internal/ui/styles/symbols.go`, `internal/ui/static/table.go`, `internal/ui/wizard/flows/prune.go`, and output capability plumbing.

- [x] Represent PR state with text as well as color, for example `#123 Merged`, using shared state formatting across list/prune views.
- [x] Keep status colors consistent while retaining an explicit distinction between PR state and the safety/action of pruning.
- [x] Gate OSC 8 hyperlinks on the destination's capabilities; produce plain references for redirected output or unsupported terminals. Treat color and hyperlink support as separate capabilities.
- [x] Preserve configured symbol choices, missing-cache behavior, and JSON fields.

Acceptance: open, merged, closed, and draft are distinguishable without color; redirected output has no OSC escapes; supported terminals retain links; table width budgeting includes state text.

### Phase 4 implementation results

Implemented on 2026-10-03:

- PR references now include their cached state and configured icon, for example `#123 ● Merged`. Shared state naming/style helpers keep open (green), merged (purple), closed (red), and draft (muted) consistent across tables and prune descriptions. The colorless theme also disables the previously fixed purple merged color. Unknown states retain only the reference, and missing PR numbers still produce empty table cells.
- Prune receives the original worktree's cached PR fields and displays the same reference/state palette. `Eligible` and `Force` describe the action separately; stale eligibility retains its age/reason alongside the actual PR state. No pruning eligibility or deletion logic changed.
- Hyperlinks require an explicit destination capability supplied by the output printer. Only a TTY with recognized iTerm2, WezTerm, Ghostty, kitty, or VTE >= 0.50 indicators enables OSC 8. Pipes, files, unknown terminals, dumb terminals, and unverified multiplexer sessions receive plain references. Detection uses environment indicators without terminal queries and does not consult color settings. Table commands resolve this capability once per output batch.
- Width budgeting preserves complete PR references/states while repository/branch columns can still shrink to their header widths, including at 40 cells. Existing symbols and cache/JSON fields remain intact.

Validation:

- `go test -race ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` passed.
- Targeted tests cover all four states with default/Nerd Font icons, unknown/missing states, colorless output, independent color/link policies, destination buffers/pipes, recognized/unsupported terminal indicators, consistent prune state colors, forced/stale action labels, and complete six-digit PR references at 40/80/120 cells.
- A local PTY output probe passed 36 terminal/theme/width combinations using kitty/WezTerm/Apple Terminal/dumb environment indicators, light/dark/colorless themes, and 40/80/120-cell widths. Each case checked hyperlink gating, readable state text, redirected output without OSC 8, full redirected notes, and unchanged JSON fields. This tests emitted sequences and destination detection; clicking links in real terminal applications remains a human compatibility check.

Capability references: [iTerm2 OSC 8 documentation](https://iterm2.com/documentation-escape-codes.html), [WezTerm hyperlinks](https://wezterm.org/recipes/hyperlinks.html), [Ghostty VT reference](https://ghostty.org/docs/vt/reference), [kitty hyperlink configuration](https://sw.kovidgoyal.net/kitty/conf/), and [VTE hyperlink API/version](https://gnome.pages.gitlab.gnome.org/vte/gtk4/property.Terminal.allow-hyperlink.html). The environment-based capability policy is intentionally conservative; unrecognized terminals and multiplexer versions fall back to plain references.

Phase 5 is next. Human terminal inspection/clicking, live forge compatibility, and the final shell-completion/output delivery checks remain pending.

## Phase 5 — Reduce subprocess work and completion latency

Review findings: **5, 6**.

Files: `cmd/wt/pr_refresh.go`, `cmd/wt/completions.go`, `internal/git/load.go`, `internal/git/repo.go`, and both forge implementations.

- [ ] Preserve upstream branch names from the existing batch branch-config read; remove the per-worktree `git config` lookup during refresh.
- [ ] Check forge availability/authentication once per effective forge/host/account for a refresh invocation. Keep tokens in memory only and avoid logging credentials.
- [ ] Reuse resolved forge clients/configuration while preserving multi-account and self-hosted behavior.
- [ ] Load label-scoped completions with bounded concurrency, stable aggregation, sorting, and deduplication. Reuse `ListWorktreesForRepos` where possible; avoid the full metadata loader for completion.
- [ ] Preserve partial-success behavior and stop scheduling unnecessary work after cancellation.

Acceptance and measurement:

- Instrument fake Git/forge executables to assert subprocess counts: auth checks scale with unique authentication contexts, upstream reads with repositories, and PR queries with eligible branches.
- Verify local/upstream branch-name differences, missing tools, failed authentication, absent repositories, and cancellation.
- Measure label completion across 1, 10, and 50 local fixture repos and PR refresh with deterministic fake latency. Record median/p95 duration and subprocess counts before/after on the same machine.
- Assert the concurrency ceiling and identical completion contents despite randomized completion order. Keep real network timings separate from deterministic regression checks.

## Delivery and validation

Suggested change sequence: dependency baseline → selection/key handling → asynchronous loading → responsive layout → status formatting → subprocess/completion optimization. Update this document as each stage finishes.

For behavior changes, add targeted regressions for the acceptance cases, then run `go test -race ./...`, `go vet ./...`, and `go build ./...`. Use isolated repository fixtures. Do not run remote-mutating integration tests as part of a dependency smoke test; perform those separately with the configured test repositories when needed.

Manually exercise checkout, PR checkout, cd, and prune wizards across small/large terminals and light/dark/no-color configurations before completing the UI phases. Exercise shell completion and stdout redirection after command/output changes.

### Delivery verification — 2026-10-04

Phases 0–4 are included in this delivery; phase 5 remains planned. Final review fixed an ANSI-styled empty description consuming a list row and added a regression. README usage now includes interactive PR checkout.

- Unit/race tests, vet, build, module verification, and diff checks passed.
- Generated bash/zsh/fish completion scripts on the completed build; bash/zsh syntax checks passed. Runtime completion offers the PR checkout `--interactive` flag, and generated scripts keep stderr separate from stdout.
- Re-ran 36 wizard PTY cases, the display-only progress stdin probe, and 36 table capability/theme/width cases with redirected output and JSON checks; all passed.
- One 123-case local command integration run passed with race detection and eight parallel test slots. A three-run repeat passed 368 cases and failed one during fixture setup because `/usr/bin/git` (Apple Git 2.50.1) segfaulted in `git config user.email`; that case passed in the other two repetitions. A single-slot run is being used to check the suite independently of that parallel subprocess failure. Live forge mutation tests were not invoked locally.

The dependency baseline and behavior changes are committed separately on the feature branch and are being delivered through a squash PR. Human terminal/click compatibility checks and phase 5 benchmarks remain pending.

### Dependency refresh results

Completed on macOS arm64 with Go 1.26.0:

- `go build ./...` — passed; no application API compatibility edits required.
- `go test -race ./...` — passed across all packages.
- `go vet ./...` — passed.
- `go mod verify` — all modules verified.
- `git diff --check` — passed.

The dependency baseline is isolated in its own feature-branch commit (`go.mod` and `go.sum`); application behavior changes are recorded in the subsequent phase results above. Live forge integration tests and manual terminal compatibility checks were not run. Existing user deletions and unrelated files were left untouched.
