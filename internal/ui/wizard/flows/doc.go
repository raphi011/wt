// Package flows provides command-specific wizard implementations.
//
// Each flow is a complete interactive wizard for a specific wt command.
// Flows use the framework and steps packages to build multi-step
// interactive experiences.
//
// Available flows:
//   - [CheckoutInteractive]: Branch checkout wizard
//   - [CdInteractive]: Change directory wizard
//   - [PruneInteractive]: Worktree pruning wizard
//   - [PrCheckoutInteractive]: PR checkout wizard
package flows
