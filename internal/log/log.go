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
}

// New creates a new logger.
func New(out io.Writer, verbose bool) *Logger {
	return &Logger{out: out, verbose: verbose}
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
	fmt.Fprintf(l.out, format, args...)
}

// Println writes a line of output.
func (l *Logger) Println(args ...any) {
	fmt.Fprintln(l.out, args...)
}

// Command logs an external command execution.
// Only prints when verbose mode is enabled.
func (l *Logger) Command(name string, args ...string) {
	if l.verbose {
		fmt.Fprintf(l.out, "$ %s %s\n", name, strings.Join(args, " "))
	}
}

// Verbose returns true if verbose mode is enabled.
func (l *Logger) Verbose() bool {
	return l.verbose
}

// Writer returns the underlying writer.
func (l *Logger) Writer() io.Writer {
	return l.out
}
