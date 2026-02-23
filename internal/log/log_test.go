package log

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestPrintf(t *testing.T) {
	t.Parallel()

	t.Run("writes formatted output", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, false, false)
		l.Printf("hello %s %d", "world", 42)
		if got := buf.String(); got != "hello world 42" {
			t.Errorf("Printf output = %q, want %q", got, "hello world 42")
		}
	})

	t.Run("suppressed when quiet", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, false, true)
		l.Printf("should not appear")
		if buf.Len() != 0 {
			t.Errorf("Printf wrote %q when quiet", buf.String())
		}
	})
}

func TestPrintln(t *testing.T) {
	t.Parallel()

	t.Run("writes line output", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, false, false)
		l.Println("hello", "world")
		if got := buf.String(); got != "hello world\n" {
			t.Errorf("Println output = %q, want %q", got, "hello world\n")
		}
	})

	t.Run("suppressed when quiet", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, false, true)
		l.Println("should not appear")
		if buf.Len() != 0 {
			t.Errorf("Println wrote %q when quiet", buf.String())
		}
	})
}

func TestCommand(t *testing.T) {
	t.Parallel()

	t.Run("verbose with dir", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, true, false)
		done := l.Command("/tmp", "git", "status")
		done(100 * time.Millisecond)
		got := buf.String()
		if !strings.Contains(got, "[/tmp] $ git status") {
			t.Errorf("Command output = %q, want to contain %q", got, "[/tmp] $ git status")
		}
		if !strings.Contains(got, "100ms") {
			t.Errorf("Command output = %q, want to contain duration", got)
		}
	})

	t.Run("verbose without dir", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, true, false)
		done := l.Command("", "echo", "hi")
		done(50 * time.Millisecond)
		got := buf.String()
		if !strings.HasPrefix(got, "$ echo hi") {
			t.Errorf("Command output = %q, want prefix %q", got, "$ echo hi")
		}
	})

	t.Run("not verbose is no-op", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, false, false)
		done := l.Command("/tmp", "git", "status")
		done(100 * time.Millisecond)
		if buf.Len() != 0 {
			t.Errorf("Command wrote %q when not verbose", buf.String())
		}
	})

	t.Run("quiet overrides verbose", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, true, true)
		done := l.Command("/tmp", "git", "status")
		done(100 * time.Millisecond)
		if buf.Len() != 0 {
			t.Errorf("Command wrote %q when quiet", buf.String())
		}
	})
}

func TestDebug(t *testing.T) {
	t.Parallel()

	t.Run("verbose key-val format", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, true, false)
		l.Debug("creating worktree", "branch", "main", "path", "/tmp")
		got := buf.String()
		if !strings.Contains(got, "creating worktree") {
			t.Errorf("Debug output = %q, want to contain message", got)
		}
		if !strings.Contains(got, "branch=main") {
			t.Errorf("Debug output = %q, want to contain branch=main", got)
		}
		if !strings.Contains(got, "path=/tmp") {
			t.Errorf("Debug output = %q, want to contain path=/tmp", got)
		}
	})

	t.Run("odd keyvals drops last", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, true, false)
		l.Debug("msg", "key1", "val1", "orphan")
		got := buf.String()
		// Only complete pairs are printed
		if !strings.Contains(got, "key1=val1") {
			t.Errorf("Debug output = %q, want to contain key1=val1", got)
		}
		if strings.Contains(got, "orphan") {
			t.Errorf("Debug output = %q, should not contain orphan key", got)
		}
	})

	t.Run("not verbose is silent", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, false, false)
		l.Debug("should not appear", "key", "val")
		if buf.Len() != 0 {
			t.Errorf("Debug wrote %q when not verbose", buf.String())
		}
	})

	t.Run("quiet overrides verbose", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, true, true)
		l.Debug("should not appear")
		if buf.Len() != 0 {
			t.Errorf("Debug wrote %q when quiet", buf.String())
		}
	})
}

func TestIsVerbose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		verbose bool
		quiet   bool
		want    bool
	}{
		{"verbose only", true, false, true},
		{"quiet only", false, true, false},
		{"both", true, true, false},
		{"neither", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l := New(io.Discard, tt.verbose, tt.quiet)
			if got := l.IsVerbose(); got != tt.want {
				t.Errorf("IsVerbose() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriter(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := New(&buf, false, false)
	if l.Writer() != &buf {
		t.Error("Writer() did not return the underlying writer")
	}
}

func TestWithLogger_FromContext(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		l := New(&buf, true, false)
		ctx := WithLogger(context.Background(), l)
		got := FromContext(ctx)
		if got != l {
			t.Error("FromContext did not return the stored logger")
		}
	})

	t.Run("fallback discard logger", func(t *testing.T) {
		t.Parallel()
		l := FromContext(context.Background())
		if l == nil {
			t.Fatal("FromContext returned nil for empty context")
		}
		// Should write to discard — verify it doesn't panic
		l.Printf("should not appear anywhere")
		l.Debug("should not appear anywhere")
		if l.Writer() != io.Discard {
			t.Error("fallback logger should write to io.Discard")
		}
	})
}
