package output

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestWithPrinter_FromContext(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		ctx := WithPrinter(context.Background(), &buf)
		p := FromContext(ctx)
		if p == nil {
			t.Fatal("FromContext returned nil")
		}
		if p.Writer() != &buf {
			t.Error("Writer() should return the buffer passed to WithPrinter")
		}
	})

	t.Run("default to stdout when not set", func(t *testing.T) {
		t.Parallel()
		p := FromContext(context.Background())
		if p == nil {
			t.Fatal("FromContext returned nil on empty context")
		}
		if p.Writer() != os.Stdout {
			t.Error("Writer() should default to os.Stdout")
		}
	})
}

func TestPrinter_Print(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	p := FromContext(WithPrinter(context.Background(), &buf))

	p.Print("hello", " ", "world")
	if got := buf.String(); got != "hello world" {
		t.Errorf("Print() wrote %q, want %q", got, "hello world")
	}
}

func TestPrinter_Printf(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	p := FromContext(WithPrinter(context.Background(), &buf))

	p.Printf("count: %d", 42)
	if got := buf.String(); got != "count: 42" {
		t.Errorf("Printf() wrote %q, want %q", got, "count: 42")
	}
}

func TestPrinter_Println(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	p := FromContext(WithPrinter(context.Background(), &buf))

	p.Println("line one")
	p.Println("line two")
	want := "line one\nline two\n"
	if got := buf.String(); got != want {
		t.Errorf("Println() wrote %q, want %q", got, want)
	}
}

func TestPrinter_Writer(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := WithPrinter(context.Background(), &buf)
	p := FromContext(ctx)

	w := p.Writer()
	if w != &buf {
		t.Error("Writer() should return the underlying writer")
	}

	// Write directly through the writer
	if _, err := w.Write([]byte("direct")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got := buf.String(); got != "direct" {
		t.Errorf("direct Write produced %q, want %q", got, "direct")
	}
}
