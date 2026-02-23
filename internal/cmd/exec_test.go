package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/raphi011/wt/internal/log"
)

func logCtx() context.Context {
	l := log.New(&bytes.Buffer{}, false, false)
	return log.WithLogger(context.Background(), l)
}

func TestRunContext_Success(t *testing.T) {
	t.Parallel()
	err := RunContext(logCtx(), "", "echo", "hello")
	if err != nil {
		t.Errorf("RunContext(echo hello) = %v, want nil", err)
	}
}

func TestRunContext_Failure(t *testing.T) {
	t.Parallel()
	err := RunContext(logCtx(), "", "sh", "-c", "exit 1")
	if err == nil {
		t.Error("RunContext(exit 1) = nil, want error")
	}
}

func TestRunContext_StderrMessage(t *testing.T) {
	t.Parallel()
	err := RunContext(logCtx(), "", "sh", "-c", "echo 'bad thing' >&2; exit 1")
	if err == nil {
		t.Fatal("RunContext = nil, want error")
	}
	if err.Error() != "bad thing" {
		t.Errorf("RunContext error = %q, want %q", err.Error(), "bad thing")
	}
}

func TestRunContext_ContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(logCtx())
	cancel()
	err := RunContext(ctx, "", "sleep", "10")
	if err == nil {
		t.Error("RunContext with cancelled context = nil, want error")
	}
	if err != context.Canceled {
		t.Errorf("RunContext error = %v, want context.Canceled", err)
	}
}

func TestRunContext_Dir(t *testing.T) {
	t.Parallel()
	// Verify command runs in specified directory
	err := RunContext(logCtx(), "/tmp", "pwd")
	if err != nil {
		t.Errorf("RunContext with dir = %v, want nil", err)
	}
}

func TestOutputContext_Success(t *testing.T) {
	t.Parallel()
	out, err := OutputContext(logCtx(), "", "echo", "hello")
	if err != nil {
		t.Fatalf("OutputContext(echo hello) = %v, want nil", err)
	}
	if got := string(out); got != "hello\n" {
		t.Errorf("OutputContext output = %q, want %q", got, "hello\n")
	}
}

func TestOutputContext_Failure(t *testing.T) {
	t.Parallel()
	_, err := OutputContext(logCtx(), "", "sh", "-c", "exit 1")
	if err == nil {
		t.Error("OutputContext(exit 1) = nil, want error")
	}
}

func TestOutputContext_StderrMessage(t *testing.T) {
	t.Parallel()
	_, err := OutputContext(logCtx(), "", "sh", "-c", "echo 'error msg' >&2; exit 1")
	if err == nil {
		t.Fatal("OutputContext = nil, want error")
	}
	if err.Error() != "error msg" {
		t.Errorf("OutputContext error = %q, want %q", err.Error(), "error msg")
	}
}

func TestOutputContext_ContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(logCtx())
	cancel()
	_, err := OutputContext(ctx, "", "sleep", "10")
	if err == nil {
		t.Error("OutputContext with cancelled context = nil, want error")
	}
	if err != context.Canceled {
		t.Errorf("OutputContext error = %v, want context.Canceled", err)
	}
}
