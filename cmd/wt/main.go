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

	// Apply config defaults to commands with Dir field
	applyConfigDefaults(&cli, &cfg)

	err = ctx.Run(&Context{Config: &cfg})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Run 'wt -h' for help")
		os.Exit(1)
	}
}

// applyConfigDefaults sets Dir from config when not specified via flag/env.
func applyConfigDefaults(cli *CLI, cfg *config.Config) {
	if cfg == nil {
		return
	}

	// Helper to set dir if empty
	setDir := func(dir *string) {
		if *dir == "" {
			*dir = cfg.WorktreeDir
		}
	}
	setFormat := func(format *string) {
		if *format == "" {
			*format = cfg.WorktreeFormat
		}
	}

	// Apply defaults based on which command was selected
	// Kong has already parsed the command, so we check each field
	setDir(&cli.Add.Dir)
	setDir(&cli.Prune.Dir)
	setDir(&cli.List.Dir)
	setDir(&cli.Show.Dir)
	setDir(&cli.Exec.Dir)
	setDir(&cli.Cd.Dir)
	setDir(&cli.Mv.Dir)
	setFormat(&cli.Mv.Format)
	setDir(&cli.Note.Set.Dir)
	setDir(&cli.Note.Get.Dir)
	setDir(&cli.Note.Clear.Dir)
	setDir(&cli.Hook.Dir)
	setDir(&cli.Pr.Open.Dir)
	setDir(&cli.Pr.Clone.Dir)
	setDir(&cli.Pr.Merge.Dir)
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
