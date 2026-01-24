package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"github.com/raphi011/wt/internal/config"
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

	// Apply format default from config to MvCmd
	if cli.Mv.Format == "" {
		cli.Mv.Format = cfg.WorktreeFormat
	}

	err = ctx.Run(&Context{Config: &cfg, WorkDir: workDir, Stdout: os.Stdout})
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
