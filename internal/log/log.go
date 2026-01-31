// Package log provides context-aware logging for wt.
package log

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type ctxKey struct{}

// Logger provides output and verbose command logging.
type Logger struct {
	out     io.Writer
	verbose bool
	quiet   bool
}

// New creates a new logger.
func New(out io.Writer, verbose, quiet bool) *Logger {
	return &Logger{out: out, verbose: verbose, quiet: quiet}
}

// WithLogger attaches a logger to the context.
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext retrieves the logger from context.
// Returns a no-op logger if none is attached.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(ctxKey{}).(*Logger); ok {
		return l
	}
	return &Logger{out: io.Discard}
}

// Printf writes formatted output.
func (l *Logger) Printf(format string, args ...any) {
	if l.quiet {
		return
	}
	fmt.Fprintf(l.out, format, args...)
}

// Println writes a line of output.
func (l *Logger) Println(args ...any) {
	if l.quiet {
		return
	}
	fmt.Fprintln(l.out, args...)
}

// Command logs an external command execution.
// Only prints when verbose mode is enabled and quiet mode is disabled.
func (l *Logger) Command(name string, args ...string) {
	if l.verbose && !l.quiet {
		fmt.Fprintf(l.out, "$ %s %s\n", name, strings.Join(args, " "))
	}
}

// Debug logs a debug message with key-value pairs.
// Only prints when verbose mode is enabled and quiet mode is disabled.
func (l *Logger) Debug(msg string, keyvals ...any) {
	if l.verbose && !l.quiet {
		var sb strings.Builder
		sb.WriteString(msg)
		for i := 0; i < len(keyvals); i += 2 {
			if i+1 < len(keyvals) {
				sb.WriteString(fmt.Sprintf(" %v=%v", keyvals[i], keyvals[i+1]))
			}
		}
		fmt.Fprintln(l.out, sb.String())
	}
}

// Verbose returns true if verbose mode is enabled.
func (l *Logger) Verbose() bool {
	return l.verbose
}

// Quiet returns true if quiet mode is enabled.
func (l *Logger) Quiet() bool {
	return l.quiet
}

// Writer returns the underlying writer.
func (l *Logger) Writer() io.Writer {
	return l.out
}
