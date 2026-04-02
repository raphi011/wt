package main

import (
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

func TestUnknownHookError(t *testing.T) {
	t.Parallel()

	t.Run("no hooks configured", func(t *testing.T) {
		t.Parallel()
		err := unknownHookError("missing", map[string]config.Hook{})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "no hooks configured") {
			t.Errorf("error = %q, want containing 'no hooks configured'", err.Error())
		}
	})

	t.Run("with available hooks", func(t *testing.T) {
		t.Parallel()
		hooks := map[string]config.Hook{
			"setup": {Command: "npm install"},
			"lint":  {Command: "golangci-lint run"},
		}
		err := unknownHookError("missing", hooks)
		if err == nil {
			t.Fatal("expected error")
		}
		errMsg := err.Error()
		if !strings.Contains(errMsg, `unknown hook "missing"`) {
			t.Errorf("error = %q, want containing 'unknown hook \"missing\"'", errMsg)
		}
		if !strings.Contains(errMsg, "available:") {
			t.Errorf("error = %q, want containing 'available:'", errMsg)
		}
	})

	t.Run("nil hooks map", func(t *testing.T) {
		t.Parallel()
		err := unknownHookError("missing", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "no hooks configured") {
			t.Errorf("error = %q, want containing 'no hooks configured'", err.Error())
		}
	})
}
