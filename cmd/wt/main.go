package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
)

func main() {
	var cli CLI

	// Load config early so we can use it for defaults
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Get working directory for context
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "wt: failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	parser, err := kong.New(&cli,
		kong.Name("wt"),
		kong.Description("Git worktree manager with GitHub/GitLab integration"),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.ExplicitGroups([]kong.Group{
			{Key: "pr", Title: "PR Commands"},
			{Key: "util", Title: "Utilities"},
			{Key: "config", Title: "Configuration"},
		}),
		kong.Vars{
			"version": versionString(),
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wt: %v\n", err)
		os.Exit(1)
	}

	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		// No command given - show help instead of error
		if len(os.Args) == 1 {
			var parseErr *kong.ParseError
			if errors.As(err, &parseErr) {
				_ = parseErr.Context.PrintUsage(false)
				return
			}
		}
		parser.FatalIfErrorf(err)
	}

	// Validate mutually exclusive flags
	if cli.Verbose && cli.Quiet {
		fmt.Fprintln(os.Stderr, "wt: --verbose and --quiet are mutually exclusive")
		os.Exit(1)
	}

	// Apply format default from config to MvCmd
	if cli.Mv.Format == "" {
		cli.Mv.Format = cfg.WorktreeFormat
	}

	// Create context with signal handling for graceful cancellation
	bgCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create logger with verbose/quiet mode from CLI flags (stderr for diagnostics)
	logger := log.New(os.Stderr, cli.Verbose, cli.Quiet)
	bgCtx = log.WithLogger(bgCtx, logger)

	// Add output printer for stdout
	bgCtx = output.WithPrinter(bgCtx, os.Stdout)

	// Inject dependencies into all commands
	injectDeps(&cli, &cfg, workDir)

	ctx.BindTo(bgCtx, (*context.Context)(nil))
	err = ctx.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Run 'wt -h' for help")
		os.Exit(1)
	}
}

// versionString returns the version string.
func versionString() string {
	return fmt.Sprintf("wt %s (%s, %s)", version, commit[:min(7, len(commit))], date)
}

// BeforeApply runs before the command and handles --version flag.
func (v VersionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	fmt.Println(vars["version"])
	app.Exit(0)
	return nil
}

// injectDeps injects the Deps struct into all commands.
func injectDeps(cli *CLI, cfg *config.Config, workDir string) {
	deps := Deps{Config: cfg, WorkDir: workDir}

	// Core commands
	cli.Checkout.Deps = deps
	cli.List.Deps = deps
	cli.Show.Deps = deps
	cli.Prune.Deps = deps
	cli.Repos.Deps = deps

	// PR commands
	cli.Pr.Checkout.Deps = deps
	cli.Pr.Create.Deps = deps
	cli.Pr.Merge.Deps = deps
	cli.Pr.View.Deps = deps

	// Utility commands
	cli.Exec.Deps = deps
	cli.Cd.Deps = deps
	cli.Mv.Deps = deps
	cli.Hook.Deps = deps

	// Note subcommands
	cli.Note.Set.Deps = deps
	cli.Note.Get.Deps = deps
	cli.Note.Clear.Deps = deps

	// Label subcommands
	cli.Label.Add.Deps = deps
	cli.Label.Remove.Deps = deps
	cli.Label.List.Deps = deps
	cli.Label.Clear.Deps = deps

	// Config commands
	cli.Config.Init.Deps = deps
	cli.Config.Show.Deps = deps
	cli.Config.Hooks.Deps = deps
	cli.Completion.Deps = deps
	cli.Init.Deps = deps
	cli.Doctor.Deps = deps
}
