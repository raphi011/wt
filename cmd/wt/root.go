package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
)

var (
	// Global flags
	verbose bool
	quiet   bool

	// Shared state injected into commands
	cfg     *config.Config
	workDir string
)

// Command group IDs for organizing help output
const (
	GroupCore     = "core"
	GroupRegistry = "registry"
	GroupPR       = "pr"
	GroupUtility  = "utility"
	GroupConfig   = "config"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "Git worktree manager with GitHub/GitLab integration",
	Long: `wt is a CLI tool for managing git worktrees with GitHub/GitLab MR integration.

It helps you manage multiple worktrees across repositories, create PRs,
and streamline your development workflow.`,
	SilenceUsage:               true,
	SilenceErrors:              true,
	SuggestionsMinimumDistance: 2, // Enable typo suggestions
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip git check for completion and help commands
		if cmd.Name() == "completion" || cmd.Name() == "__complete" || cmd.Name() == "help" {
			return nil
		}

		// Validate mutually exclusive flags
		if verbose && quiet {
			return fmt.Errorf("--verbose and --quiet are mutually exclusive")
		}

		// Check git is available
		return git.CheckGit()
	},
	// Run is not set - shows help when no subcommand provided
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	// Load config
	loadedCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	cfg = &loadedCfg

	// Get working directory
	workDir, err = os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "wt: failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	// Create context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create logger (stderr for diagnostics)
	logger := log.New(os.Stderr, verbose, quiet)
	ctx = log.WithLogger(ctx, logger)

	// Add output printer (stdout for primary data)
	ctx = output.WithPrinter(ctx, os.Stdout)

	// Store context for commands to use
	rootCmd.SetContext(ctx)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Run 'wt -h' for help")
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show external commands being executed")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress all log output")
	rootCmd.MarkFlagsMutuallyExclusive("verbose", "quiet")

	// Version flag
	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	// Add command groups for organized help output
	rootCmd.AddGroup(
		&cobra.Group{ID: GroupCore, Title: "Core Commands:"},
		&cobra.Group{ID: GroupRegistry, Title: "Registry Commands:"},
		&cobra.Group{ID: GroupPR, Title: "Pull Request Commands:"},
		&cobra.Group{ID: GroupUtility, Title: "Utility Commands:"},
		&cobra.Group{ID: GroupConfig, Title: "Configuration Commands:"},
	)

	// Core commands
	rootCmd.AddCommand(newCheckoutCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newPruneCmd())
	rootCmd.AddCommand(newReposCmd())

	// Registry commands
	rootCmd.AddCommand(newAddCmd())
	rootCmd.AddCommand(newRemoveCmd())
	rootCmd.AddCommand(newCloneCmd())

	// PR commands
	rootCmd.AddCommand(newPrCmd())

	// Utility commands
	rootCmd.AddCommand(newExecCmd())
	rootCmd.AddCommand(newCdCmd())
	rootCmd.AddCommand(newNoteCmd())
	rootCmd.AddCommand(newLabelCmd())
	rootCmd.AddCommand(newHookCmd())

	// Config commands
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newCompletionCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newDoctorCmd())
}
