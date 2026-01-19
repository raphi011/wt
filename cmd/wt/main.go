package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexflint/go-arg"

	"github.com/raphi011/wt/internal/config"
)

func main() {
	var args Args
	p, err := arg.NewParser(arg.Config{StrictSubcommands: true}, &args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := p.Parse(os.Args[1:]); err != nil {
		switch err {
		case arg.ErrHelp:
			writeHelp(os.Stdout, p, &args)
			os.Exit(0)
		case arg.ErrVersion:
			fmt.Println(args.Version())
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			p.WriteUsage(os.Stderr)
			os.Exit(1)
		}
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Apply config defaults
	if args.Create != nil && args.Create.Dir == "" {
		args.Create.Dir = cfg.DefaultPath
	}
	if args.Open != nil && args.Open.Dir == "" {
		args.Open.Dir = cfg.DefaultPath
	}
	if args.Tidy != nil && args.Tidy.Dir == "" {
		args.Tidy.Dir = cfg.DefaultPath
	}
	if args.List != nil && args.List.Dir == "" {
		args.List.Dir = cfg.DefaultPath
	}
	if args.Exec != nil && args.Exec.Dir == "" {
		args.Exec.Dir = cfg.DefaultPath
	}
	if args.Pr != nil && args.Pr.Open != nil && args.Pr.Open.Dir == "" {
		args.Pr.Open.Dir = cfg.DefaultPath
	}
	if args.Pr != nil && args.Pr.Clone != nil && args.Pr.Clone.Dir == "" {
		args.Pr.Clone.Dir = cfg.DefaultPath
	}
	if args.Pr != nil && args.Pr.Refresh != nil && args.Pr.Refresh.Dir == "" {
		args.Pr.Refresh.Dir = cfg.DefaultPath
	}
	if args.Mv != nil {
		if args.Mv.Dir == "" {
			args.Mv.Dir = cfg.DefaultPath
		}
		if args.Mv.Format == "" {
			args.Mv.Format = cfg.WorktreeFormat
		}
	}

	switch {
	case args.Create != nil:
		if err := runCreate(args.Create, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Open != nil:
		if err := runOpen(args.Open, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Tidy != nil:
		if err := runTidy(args.Tidy, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.List != nil:
		if err := runList(args.List); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Exec != nil:
		if err := runExec(args.Exec); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Mv != nil:
		if err := runMv(args.Mv, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Note != nil:
		if err := runNote(args.Note); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Completion != nil:
		if err := runCompletion(args.Completion); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Pr != nil:
		if err := runPr(args.Pr, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Config != nil:
		if err := runConfig(args.Config, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		p.WriteHelp(os.Stdout)
		os.Exit(1)
	}
}

// writeHelp prints help with subcommand-specific descriptions
func writeHelp(w *os.File, p *arg.Parser, args *Args) {
	// Determine active subcommand and get its description
	var desc string
	switch {
	case args.Create != nil:
		desc = args.Create.Description()
	case args.Open != nil:
		desc = args.Open.Description()
	case args.Tidy != nil:
		desc = args.Tidy.Description()
	case args.List != nil:
		desc = args.List.Description()
	case args.Exec != nil:
		desc = args.Exec.Description()
	case args.Mv != nil:
		desc = args.Mv.Description()
	case args.Note != nil:
		if args.Note.Set != nil {
			desc = args.Note.Set.Description()
		} else if args.Note.Get != nil {
			desc = args.Note.Get.Description()
		} else if args.Note.Clear != nil {
			desc = args.Note.Clear.Description()
		} else {
			desc = args.Note.Description()
		}
	case args.Pr != nil:
		if args.Pr.Open != nil {
			desc = args.Pr.Open.Description()
		} else if args.Pr.Clone != nil {
			desc = args.Pr.Clone.Description()
		} else if args.Pr.Refresh != nil {
			desc = args.Pr.Refresh.Description()
		} else if args.Pr.Merge != nil {
			desc = args.Pr.Merge.Description()
		} else {
			desc = args.Pr.Description()
		}
	case args.Config != nil:
		if args.Config.Init != nil {
			desc = args.Config.Init.Description()
		} else if args.Config.Show != nil {
			desc = args.Config.Show.Description()
		} else if args.Config.Hooks != nil {
			desc = args.Config.Hooks.Description()
		} else {
			desc = args.Config.Description()
		}
	case args.Completion != nil:
		desc = args.Completion.Description()
	default:
		// No subcommand - use custom root help
		writeRootHelp(w)
		return
	}

	// Print subcommand description, then full help (which includes usage + flags)
	// Capture WriteHelp output and replace the parent description with subcommand's
	fmt.Fprintln(w, desc)
	fmt.Fprintln(w)

	// WriteHelp outputs: description, version line, usage, options
	// We need usage + options, so capture and skip first lines
	var buf strings.Builder
	p.WriteHelp(&buf)
	lines := strings.Split(buf.String(), "\n")

	// Find "Usage:" line and print from there
	for i, line := range lines {
		if strings.HasPrefix(line, "Usage:") {
			fmt.Fprintln(w, strings.Join(lines[i:], "\n"))
			break
		}
	}
}

// writeRootHelp prints custom help for the root command
func writeRootHelp(w *os.File) {
	fmt.Fprintln(w, "wt - Git worktree manager with GitHub/GitLab integration")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: wt <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  create      Create worktree for new branch")
	fmt.Fprintln(w, "  open        Open worktree for existing branch")
	fmt.Fprintln(w, "  tidy        Remove merged worktrees")
	fmt.Fprintln(w, "  list        List worktrees with stable IDs")
	fmt.Fprintln(w, "  exec        Run command in worktree by ID")
	fmt.Fprintln(w, "  mv          Move worktrees")
	fmt.Fprintln(w, "  note        Manage branch notes")
	fmt.Fprintln(w, "  pr          Work with PRs")
	fmt.Fprintln(w, "  config      Manage configuration")
	fmt.Fprintln(w, "  completion  Generate shell completions")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -h, --help     Show help")
	fmt.Fprintln(w, "      --version  Show version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  wt create feature-x          Create worktree for new branch")
	fmt.Fprintln(w, "  wt pr refresh && wt tidy -n  Refresh PR status, preview tidy")
	fmt.Fprintln(w, "  wt pr open 123               Checkout PR #123 as worktree")
}

// expandPath expands ~ to home directory
func expandPath(path string) (string, error) {
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return home, nil
	}
	return path, nil
}
