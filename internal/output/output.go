// Package output provides context-aware output for wt.
// Stdout is used for primary data output (tables, paths, JSON).
// Stderr (via log package) is used for diagnostics.
package output

import (
	"context"
	"fmt"
	"io"
	"os"
)

type ctxKey struct{}

// Printer writes primary output (data, tables, paths, JSON) to stdout.
type Printer struct {
	w io.Writer
}

// New creates a new Printer writing to the given writer.
func New(w io.Writer) *Printer {
	return &Printer{w: w}
}

// WithPrinter attaches a Printer to the context.
func WithPrinter(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, ctxKey{}, &Printer{w: w})
}

// FromContext retrieves the Printer from context.
// Returns a Printer writing to os.Stdout if none is attached.
func FromContext(ctx context.Context) *Printer {
	if p, ok := ctx.Value(ctxKey{}).(*Printer); ok {
		return p
	}
	return &Printer{w: os.Stdout}
}

// Print writes output without a newline.
func (p *Printer) Print(a ...any) {
	fmt.Fprint(p.w, a...)
}

// Printf writes formatted output.
func (p *Printer) Printf(format string, a ...any) {
	fmt.Fprintf(p.w, format, a...)
}

// Println writes a line of output.
func (p *Printer) Println(a ...any) {
	fmt.Fprintln(p.w, a...)
}

// Writer returns the underlying writer.
func (p *Printer) Writer() io.Writer {
	return p.w
}
