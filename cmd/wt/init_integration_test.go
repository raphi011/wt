//go:build integration

package main

import (
	"strings"
	"testing"
)

// TestInit_Bash tests that init bash succeeds and outputs a shell wrapper.
//
// Scenario: User runs `wt init bash`
// Expected: Command succeeds without error
func TestInit_Bash(t *testing.T) {
	t.Parallel()

	ctx := testContext(t)
	cmd := newInitCmd()

	// init outputs via fmt.Print to real stdout, so we can only verify no error
	_, err := executeCommand(ctx, cmd, "bash")
	if err != nil {
		t.Fatalf("init bash failed: %v", err)
	}
}

// TestInit_Zsh tests that init zsh succeeds and outputs a shell wrapper.
//
// Scenario: User runs `wt init zsh`
// Expected: Command succeeds without error
func TestInit_Zsh(t *testing.T) {
	t.Parallel()

	ctx := testContext(t)
	cmd := newInitCmd()

	_, err := executeCommand(ctx, cmd, "zsh")
	if err != nil {
		t.Fatalf("init zsh failed: %v", err)
	}
}

// TestInit_Fish tests that init fish succeeds and outputs a shell wrapper.
//
// Scenario: User runs `wt init fish`
// Expected: Command succeeds without error
func TestInit_Fish(t *testing.T) {
	t.Parallel()

	ctx := testContext(t)
	cmd := newInitCmd()

	_, err := executeCommand(ctx, cmd, "fish")
	if err != nil {
		t.Fatalf("init fish failed: %v", err)
	}
}

// TestInit_UnsupportedShell tests error for an unsupported shell.
//
// Scenario: User runs `wt init powershell`
// Expected: Returns error about unsupported shell
func TestInit_UnsupportedShell(t *testing.T) {
	t.Parallel()

	ctx := testContext(t)
	cmd := newInitCmd()

	_, err := executeCommand(ctx, cmd, "powershell")
	if err == nil {
		t.Fatal("expected error for unsupported shell, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("expected 'unsupported shell' error, got %q", err.Error())
	}
}
