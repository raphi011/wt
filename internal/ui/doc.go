// Package ui provides terminal UI components for wt command output.
//
// This package is organized into subpackages by functionality:
//
// # Subpackages
//
// The UI package is structured as follows:
//
//   - [static]: Non-interactive output (tables, formatted text)
//   - [progress]: Progress indicators (spinners, progress bars)
//   - [prompt]: Simple interactive prompts (confirm, text input, select)
//   - [styles]: Shared lipgloss styles for visual consistency
//   - [wizard]: Multi-step interactive flows
//
// # Static Output
//
// Use [static.RenderTable] to render aligned tables:
//
//	import "github.com/raphi011/wt/internal/ui/static"
//
//	headers := []string{"NAME", "VALUE"}
//	rows := [][]string{{"foo", "bar"}, {"baz", "qux"}}
//	output := static.RenderTable(headers, rows)
//
// # Progress Indicators
//
// Use [progress.Spinner] for progress indication:
//
//	import "github.com/raphi011/wt/internal/ui/progress"
//
//	sp := progress.NewSpinner("Loading...")
//	sp.Start()
//	defer sp.Stop()
//
// # Simple Prompts
//
// Use [prompt] package for simple interactive prompts:
//
//	import "github.com/raphi011/wt/internal/ui/prompt"
//
//	result, err := prompt.Confirm("Continue?")
//	result, err := prompt.TextInput("Name:", "placeholder")
//	result, err := prompt.Select("Choose:", options)
//
// # Wizard Flows
//
// For complex multi-step interactions, use the wizard subpackages:
//
//   - [wizard/framework]: Core wizard orchestration
//   - [wizard/steps]: Reusable step components (FilterableListStep, SingleSelectStep, TextInputStep)
//   - [wizard/flows]: Command-specific wizard implementations
//
// Example usage:
//
//	import "github.com/raphi011/wt/internal/ui/wizard/flows"
//
//	result, err := flows.CdInteractive(params)
//	result, err := flows.CheckoutInteractive(params)
//	result, err := flows.PruneInteractive(params)
//
// # Design Notes
//
// Output is designed for terminal display with:
//   - Monospace font assumptions
//   - ANSI color support
//   - Clear separation between static and interactive components
package ui
