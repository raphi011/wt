// Package forge provides an abstraction layer for git hosting services.
//
// The package supports GitHub (via gh CLI) and GitLab (via glab CLI), enabling
// wt commands to work seamlessly with both platforms without duplicating logic.
//
// # Forge Interface
//
// The [Forge] interface defines operations for:
//
//   - Fetching PR/MR information for a branch
//   - Getting the source branch for a PR number
//   - Cloning repositories
//   - Creating, viewing, and merging PRs
//
// # Platform Detection
//
// Use [Detect] to automatically determine the forge from a repository's origin URL.
// Detection checks:
//
//  1. Custom host mappings from config (for self-hosted instances)
//  2. URL patterns (gitlab.com, gitlab.* domains)
//  3. Falls back to GitHub (most common)
//
// # Usage
//
//	f := forge.Detect(originURL, hostMap, forgeConfig)
//	pr, err := f.GetPRForBranch(originURL, branch)
//
// # Platform Differences
//
// Some features have platform-specific limitations:
//
//   - GitLab does not support rebase merge via CLI (only squash and merge)
//   - PR state names differ (OPEN/MERGED/CLOSED vs open/merged/closed)
//   - Draft PR handling varies between platforms
//
// Any feature involving forge operations must implement both GitHub and GitLab.
// Never call gh or glab directly outside this package.
package forge
