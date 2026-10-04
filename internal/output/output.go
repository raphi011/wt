// Package output provides context-aware output for wt.
// Stdout is used for primary data output (tables, paths, JSON).
// Stderr (via log package) is used for diagnostics.
package output

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

type ctxKey struct{}

// Printer writes primary output (data, tables, paths, JSON) to stdout.
type Printer struct {
	w io.Writer
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

// TerminalWidth reports the actual data destination's width. Redirected output
// deliberately has no width constraint, even when stderr is a terminal.
func (p *Printer) TerminalWidth() int {
	f, ok := p.w.(*os.File)
	if !ok || !term.IsTerminal(f.Fd()) {
		return 0
	}
	width, _, err := term.GetSize(f.Fd())
	if err != nil {
		return 0
	}
	return width
}
