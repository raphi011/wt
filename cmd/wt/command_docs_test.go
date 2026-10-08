package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var updateDocs = flag.Bool("update-docs", false, "rewrite docs/commands.md from the command tree")

const commandDocsPath = "../../docs/commands.md"

// TestCommandDocs fails when docs/commands.md no longer matches the command
// tree. Regenerate it with `just docs`.
func TestCommandDocs(t *testing.T) {
	want := renderCommandDocs(rootCmd)

	if *updateDocs {
		if err := os.WriteFile(commandDocsPath, []byte(want), 0o644); err != nil {
			t.Fatalf("write %s: %v", commandDocsPath, err)
		}
		return
	}

	got, err := os.ReadFile(commandDocsPath)
	if err != nil {
		t.Fatalf("read %s: %v", commandDocsPath, err)
	}
	if string(got) != want {
		t.Errorf("%s is out of date, run `just docs`", commandDocsPath)
	}
}

// renderCommandDocs renders the help of root and all its subcommands as one
// markdown document.
func renderCommandDocs(root *cobra.Command) string {
	var b strings.Builder
	b.WriteString("# Command Reference\n\n")
	b.WriteString("<!-- Generated from the command definitions by `just docs`. Do not edit. -->\n\n")

	cmds := documentedCommands(root)
	for _, cmd := range cmds {
		depth := strings.Count(cmd.CommandPath(), " ") - 1
		fmt.Fprintf(&b, "%s- [%s](#%s) - %s\n", strings.Repeat("  ", depth), cmd.CommandPath(), strings.ReplaceAll(cmd.CommandPath(), " ", "-"), cmd.Short)
	}

	b.WriteString("\n## Global flags\n\n")
	writeCodeBlock(&b, root.PersistentFlags().FlagUsages())

	for _, cmd := range cmds {
		heading := "##"
		if cmd.Parent() != root {
			heading = "###"
		}
		// cobra adds the help flag on first use, add it now for a stable result
		cmd.InitDefaultHelpFlag()
		fmt.Fprintf(&b, "%s %s\n\n%s\n\n", heading, cmd.CommandPath(), cmd.Short)
		writeCodeBlock(&b, cmd.UseLine())
		if len(cmd.Aliases) > 0 {
			fmt.Fprintf(&b, "Aliases: `%s`\n\n", strings.Join(cmd.Aliases, "`, `"))
		}
		if cmd.Long != "" && cmd.Long != cmd.Short {
			writeCodeBlock(&b, cmd.Long)
		}
		if cmd.Example != "" {
			b.WriteString("Examples:\n\n")
			writeCodeBlock(&b, cmd.Example)
		}
		if flags := cmd.NonInheritedFlags().FlagUsages(); flags != "" {
			b.WriteString("Flags:\n\n")
			writeCodeBlock(&b, flags)
		}
	}

	return b.String()
}

// documentedCommands returns the subcommands of cmd depth-first, in help order.
func documentedCommands(cmd *cobra.Command) []*cobra.Command {
	var cmds []*cobra.Command
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() || sub.Name() == "help" {
			continue
		}
		cmds = append(cmds, sub)
		cmds = append(cmds, documentedCommands(sub)...)
	}
	return cmds
}

func writeCodeBlock(b *strings.Builder, text string) {
	fmt.Fprintf(b, "```text\n%s\n```\n\n", strings.TrimRight(text, "\n"))
}

// TestShortFlagsHaveOneMeaning ensures a shorthand letter maps to the same
// long flag on every command.
func TestShortFlagsHaveOneMeaning(t *testing.T) {
	names := map[string]string{} // shorthand -> long flag name

	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Shorthand == "" {
				return
			}
			if name, ok := names[f.Shorthand]; ok && name != f.Name {
				t.Errorf("-%s is --%s on %q but --%s elsewhere", f.Shorthand, f.Name, cmd.CommandPath(), name)
			}
			names[f.Shorthand] = f.Name
		})
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}
